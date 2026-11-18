# 完整提交验证输入与部署失败日志修复

## 故障与根因

reading_list main `7781c72ed24ec196f0f6c7548ac23d718d4b5f14` 的本机部署在 Node 测试阶段失败。旧控制器只提取 `app/`、`deploy/` 和 `tests/`；其中推荐测试还读取同一提交中 `docs/05-validation/tasks/100/` 下的基线与目录 fixture，因而出现 ENOENT。`capture_output=True` 又使异常只显示退出码，未显示原始失败输出。

这属于验证输入被部署器截断，不是缺少需要在运行目录手工补齐的文件。

## 通用修复与交付入口

维护源为 `examples/github/local-preview`，代码版本 `d81d60b9df89b43d91c2f1a16ab21ab39789f647`。

- 验证提取目标提交完整的普通文件快照，并在该目录执行检查；拒绝链接和特殊文件。
- 公开发布仍只复制 `app/`。测试、文档和控制文件不对外发布。
- 测试 stdout/stderr 写入 Actions 日志及 `<deployment-root>/logs/validation/<sha>-*.log`；失败保留当前发布和数据。
- 安装器提供 `--controller-only`：验证仓库与端口绑定，使用部署锁、原子替换、内容哈希和旧控制器备份，保留服务和数据。没有 `preview.json` 的旧安装可按文档登记并升级。

本机使用上述提交的 Git archive 导出安装器进行升级，没有直接修改运行副本。安装后控制器 SHA-256 为 `33fc07cf873867d2fbaea902340611ad85bedfa2bf3b038c9a2c3bc2ca8d1d86`。

## 验证结果

| 验证 | 结果与证据 |
|---|---|
| 原故障重现 | 旧提取方式运行目标提交，推荐测试因 fixture 缺失失败；16 项通过，1 个测试套件失败 |
| 完整目标提交验证 | 修复后对相同目标 SHA 验证，25/25 Node 测试通过；发布目录无 docs/tests |
| 部署器回归 | 14/14 Python 测试通过，覆盖完整提交输入、未提交内容隔离、失败日志、旧版本/数据保留、健康失败回滚、取消恢复、过期计划、私有文件隔离和重复升级 |
| 已有安装升级 | 正式安装入口执行成功；服务和旧发布保持不变，随后由 Actions 发布 |
| 真实失败重试 | [Run 37212170506，attempt 2](https://github.com/big91987/reading_list/actions/runs/37212170506/attempts/2) 成功；prepare 和 deploy-reviewed 均成功 |
| 审批 | 唯一原因是脚本/配置路径变化；发布声明、内容指纹、兼容基线通过，无迁移计划。核对仅切换静态发布后，按用户修复并重新部署授权走正常 Environment 审批，未关闭或绕过规则 |
| 实际运行 | 本机 5533 首页 HTTP 200；`/__deployment.json` 返回目标 `7781c72ed24ec196f0f6c7548ac23d718d4b5f14` |
| 数据保留 | 部署根目录 data 指纹前后一致（本次为空目录）；previous 指向 `10c5e03d04bad026524e20476efd124db7e3a907`。真实浏览器仍显示原有“部署保留验证 · 人类简史”条目及已读状态；未写入或清空用户书单 |

回归命令：

```sh
python3 -m unittest discover -s examples/github/local-preview/tests -p '*_test.py' -v
ruff check examples/github/local-preview/scripts examples/github/local-preview/tests
```

服务器隔离测试需允许绑定临时 localhost 端口；在禁止 bind 的执行沙箱中出现 PermissionError，使用正常本机权限后通过，未改变测试断言。

## 明确边界

- 源修复独立 PR 交用户合并；已部署的是该明确提交，不代表源 main 已包含修复。
- 本次真实验证覆盖已有 macOS 安装升级及 Actions 重试。未在另一台全新 Mac 上安装 LaunchAgent；不能宣称全新宿主端到端验收完成。
- `--controller-only` 不重启或热更新 HTTP 服务，只让后续启动的部署验证进程使用新代码。
- 产品推荐功能的目录后端与采集服务安装不属于这次静态发布；产品发布说明也明确要求独立安装审查。真实浏览器进入“书籍推荐”显示“推荐加载失败；我的书单仍可使用”。静态部署成功不能据此声称推荐功能已完整上线。
- 持久数据目录本次为空；浏览器现有一条数据可见，不等于所有用户数据完成逐字节迁移验证。
