# Fixed Linux/amd64 bases, verified against the official registry on 112.
FROM golang:1.25.6-bookworm@sha256:3c6d396d2248ce87bf47a45a418d44f899b27c225dcc34acf57a219ee850c375 AS go
FROM node:22.22.0-bookworm@sha256:2e3d655fd1e3ffaa6b5f23ee9f3905a0fd9e8c0a65df94c8ae6e4d18a0f48870
COPY --from=go /usr/local/go /usr/local/go
ENV PATH=/opt/agent-runtime/node_modules/.bin:/usr/local/go/bin:$PATH \
    NODE_PATH=/opt/agent-runtime/node_modules \
    PLAYWRIGHT_BROWSERS_PATH=/opt/agent-runtime/browsers
RUN sed -i 's|http://deb.debian.org|https://deb.debian.org|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get -o APT::Update::Error-Mode=any -o Acquire::Retries=0 -o Acquire::http::Timeout=20 -o Acquire::https::Timeout=20 update && apt-get install -y --no-install-recommends python3 python3-venv ca-certificates git curl \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /opt/agent-runtime
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts && playwright install --with-deps chromium \
    && npm cache clean --force && rm -rf /var/lib/apt/lists/*
COPY check.py ./
RUN mkdir -p /workspace && chmod 0700 /workspace
WORKDIR /workspace
CMD ["tail", "-f", "/dev/null"]
