# Vue 管理控制台参考

本例是一个配额查询功能，可连接任意实现同一契约的后端。默认 Vite 将 /api 代理到本机 8080。Go SaaS 例子提供该接口，但前端不导入后端代码。

## 接口与状态

GET /api/quotas/{tenant} 返回 {"tenant":"demo","limit":100,"reserved":0,"used":0}。计数均为非负安全整数；不能用 JavaScript number 承载任意精度金额。HTTP 非成功状态与结构错误分别抛错。

src/app 负责组装；features/quotas 拥有页面、局部状态和业务 API；shared/api 不含业务规则。新功能独立建 feature，有跨功能协作需求再定义公开接口，不建立空的全局 store 或所有业务共用的大 service。

切换租户先取消前次请求，并用请求序号抵御不遵守取消的迟到响应。卸载 feature 会取消请求，错误与结果不会回写已卸载的状态。状态测试使用真实 Vue effectScope，不 mock Vue。

## 运行

```sh
npm ci
npm run verify
npm run dev
```

启动后访问 Vite 输出的本机地址。页面会请求真实 API，没有硬编码回包；未启动后端时显示失败和重试。生产部署需配置同源 API/反向代理和认证；这里不承诺提供完整身份体系、发布或真实产品验收。
