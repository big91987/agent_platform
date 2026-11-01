// Parse actual syntax (including type-only and re-export edges), never regex source.
const fs = require("node:fs");
const path = require("node:path");
const ts = require("typescript");
const { parse, compileTemplate } = require("@vue/compiler-sfc");
const input = JSON.parse(fs.readFileSync(0, "utf8"));
const files = [],
  errors = [],
  root = input.root;
let options = {
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  module: ts.ModuleKind.ESNext,
  allowJs: true,
};
const project = path.resolve(root, input.project_root || ".");
const configPath = path.join(project, "tsconfig.json");
if (fs.existsSync(configPath)) {
  const config = ts.readConfigFile(configPath, ts.sys.readFile);
  if (config.error)
    errors.push(
      ts.flattenDiagnosticMessageText(config.error.messageText, "\n"),
    );
  else {
    const converted = ts.convertCompilerOptionsFromJson(
      config.config.compilerOptions || {},
      project,
    );
    errors.push(
      ...converted.errors.map((e) =>
        ts.flattenDiagnosticMessageText(e.messageText, "\n"),
      ),
    );
    options = { ...options, ...converted.options };
    if (config.config.extends || config.config.references)
      errors.push(
        "extended/referenced tsconfig is not supported by this analyzer; supply a standalone config",
      );
  }
}
const manifest = path.join(project, "package.json");
const packages = fs.existsSync(manifest)
  ? JSON.parse(fs.readFileSync(manifest, "utf8"))
  : {};
const declared = {
  ...packages.dependencies,
  ...packages.devDependencies,
  ...packages.peerDependencies,
};
const sourceNames = new Set(input.files.map((f) => f.path));
function resolve(spec, source) {
  const from = path.join(root, source);
  const resolved = ts.resolveModuleName(
    spec,
    from,
    options,
    ts.sys,
  ).resolvedModule;
  if (resolved && !resolved.isExternalLibraryImport)
    return path
      .relative(root, resolved.resolvedFileName)
      .split(path.sep)
      .join("/");
  const candidates = [];
  if (spec.startsWith("."))
    candidates.push(path.resolve(path.dirname(from), spec));
  for (const [pattern, replacements] of Object.entries(options.paths || {})) {
    const [prefix, suffix] = pattern.split("*");
    const match = pattern.includes("*")
      ? spec.startsWith(prefix) && spec.endsWith(suffix || "")
      : spec === pattern;
    if (match)
      for (const replacement of replacements)
        candidates.push(
          path.resolve(
            options.baseUrl || project,
            replacement.replace(
              "*",
              spec.slice(prefix.length, suffix ? -suffix.length : undefined),
            ),
          ),
        );
  }
  for (const candidate of candidates) {
    const asset = path.relative(root, candidate).split(path.sep).join("/");
    if (
      (input.assets || []).includes(asset) &&
      /\.(css|svg|png|jpg|woff2?|json)$/.test(candidate)
    )
      return null;
    for (const ext of [
      "",
      ".ts",
      ".tsx",
      ".mts",
      ".cts",
      ".mjs",
      ".cjs",
      ".js",
      ".jsx",
      ".vue",
      "/index.ts",
      "/index.js",
    ]) {
      const name = path
        .relative(root, candidate + ext)
        .split(path.sep)
        .join("/");
      if (sourceNames.has(name)) return name;
    }
    // TypeScript resolves imports ending in .js to a source .ts file.
    if (candidate.endsWith(".js")) {
      const name = path
        .relative(root, candidate.slice(0, -3) + ".ts")
        .split(path.sep)
        .join("/");
      if (sourceNames.has(name)) return name;
    }
  }
  if (spec.startsWith(".") || candidates.length || spec.startsWith("@/"))
    throw Error(`unresolved local import ${source}: ${spec}`);
  const packageName = spec.startsWith("@")
    ? spec.split("/").slice(0, 2).join("/")
    : spec.split("/")[0];
  if (!declared[packageName] && !spec.startsWith("node:"))
    throw Error(`undeclared external import ${source}: ${spec}`);
  return null;
}
for (const file of input.files) {
  try {
    const record = {
      path: file.path,
      language: "typescript",
      unit: "ts:" + file.path,
      imports: [],
      symbols: [],
    };
    let sections = [
      {
        content: file.text,
        offset: 0,
        lang: /\.[jt]sx$/.test(file.path) ? "tsx" : "ts",
      },
    ];
    if (file.path.endsWith(".vue")) {
      const result = parse(file.text, { filename: file.path });
      if (result.errors.length)
        throw Error(result.errors.map(String).join("; "));
      if (result.descriptor.template) {
        const compiled = compileTemplate({
          source: result.descriptor.template.content,
          filename: file.path,
          id: file.path,
        });
        if (compiled.errors.length)
          throw Error(compiled.errors.map(String).join("; "));
      }
      sections = [result.descriptor.script, result.descriptor.scriptSetup]
        .filter(Boolean)
        .map((s) => {
          if (s.src) throw Error("external Vue script src is unsupported");
          return {
            content: s.content,
            offset: s.loc.start.line - 1,
            lang: s.lang || "js",
          };
        });
    }
    for (const section of sections) {
      if (!["ts", "tsx", "js", "jsx"].includes(section.lang))
        throw Error("unsupported script language " + section.lang);
      const source = ts.createSourceFile(
        file.path,
        section.content,
        ts.ScriptTarget.Latest,
        true,
        section.lang.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
      );
      if (source.parseDiagnostics.length)
        throw Error(
          source.parseDiagnostics
            .map((d) => ts.flattenDiagnosticMessageText(d.messageText, "\n"))
            .join("; "),
        );
      const line = (node) =>
        source.getLineAndCharacterOfPosition(node.getStart(source)).line +
        1 +
        section.offset;
      function add(literal) {
        record.imports.push({
          specifier: literal.text,
          line: line(literal),
          target: resolve(literal.text, file.path),
        });
      }
      function visit(node) {
        if (
          (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
          node.moduleSpecifier
        )
          add(node.moduleSpecifier);
        if (
          ts.isImportEqualsDeclaration(node) &&
          ts.isExternalModuleReference(node.moduleReference)
        )
          add(node.moduleReference.expression);
        if (ts.isImportTypeNode(node) && ts.isLiteralTypeNode(node.argument))
          add(node.argument.literal);
        if (
          ts.isCallExpression(node) &&
          (node.expression.kind === ts.SyntaxKind.ImportKeyword ||
            (ts.isIdentifier(node.expression) &&
              node.expression.text === "require"))
        ) {
          if (
            node.arguments.length !== 1 ||
            !ts.isStringLiteralLike(node.arguments[0])
          )
            throw Error("computed import/require is not statically resolved");
          add(node.arguments[0]);
        }
        if (
          (ts.isFunctionDeclaration(node) ||
            ts.isClassDeclaration(node) ||
            ts.isInterfaceDeclaration(node) ||
            ts.isTypeAliasDeclaration(node)) &&
          node.name
        )
          record.symbols.push({
            name: node.name.text,
            line: line(node),
            end:
              source.getLineAndCharacterOfPosition(node.end).line +
              1 +
              section.offset,
          });
        ts.forEachChild(node, visit);
      }
      visit(source);
    }
    files.push(record);
  } catch (error) {
    errors.push(`${file.path}: ${error.message}`);
  }
}
process.stdout.write(JSON.stringify({ files, errors }));
