# 检测规则与兼容边界

网关按 URL 中启用的 `HPSIBEG` 标记扫描请求的 JSON 值，生成可逆占位符。以下策略适用于完成网关地址设置后的检测优化；对比基线为本项目 `a3603a2` 与 Cosy Redact Gateway `0e2be2e`。

## 重叠命中

先收集所有启用规则的候选区间，再合并有交集的区间。优先级只决定覆盖完整区间的规则标签，不会丢弃较长凭据的剩余部分；仅相邻、没有交集的命中保持独立。若没有单条规则覆盖整个合并区间，使用 `SECRET` 标签。原有占位符本身保持不变，其两侧仍会检测。

例如 `qZ9vX2kL13800138000mN4pR7sT` 同时命中随机令牌和手机号时，现在会替换全部 27 个字符；旧实现启用全部规则时只替换其中 11 个字符。回归测试遍历七个标记的组合，检查新增规则不会减少覆盖。

## H：随机令牌

使用按长度校准的字符二元组模型，候选为至少 9 个 ASCII 字母或数字组成的连续片段。模型统一按小写评分，另检查字符多样性；纯数字、低多样性的重复值和 8 字符以下的片段不由 H 处理。模型更容易识别全小写随机值，同时减少 `RequestValidationError`、`CustomerInformation` 等自然词拼接标识符的误报。

同一批固定种子的合成对比样本结果：

| 样本 | 数量 | 旧 H | 优化后 H |
|---|---:|---:|---:|
| PascalCase 单词拼接的误报 | 30,000 | 64.04% | 0.99% |
| 小写单词拼接的误报 | 30,000 | 0% | 0.99% |
| 12 字符十六进制随机值检出 | 2,000 | 0.45% | 96.80% |
| 12 字符 base62 随机值检出 | 2,000 | 34.25% | 97.50% |
| 32 字符小写随机值检出 | 2,000 | 0% | 99.95% |

这些数字描述固定测试样本，不代表真实请求的检出率保证。H 仍属于启发式规则：自然词凭据可能漏检，随机业务标识符可能被替换。测试分别约束自然词、PascalCase 的误报，并按长度和字符集检查随机值检出率；较短的小写随机值使用独立门槛。

## G：服务凭据

内置从 Cosy 导出的 218 条可移植规则，以 Go RE2 编译；同时保留原有 7 条私钥、AWS、GitHub、GitLab、JWT、连接串及 Bearer 规则。覆盖 Hugging Face、细粒度 GitHub PAT、Stripe、npm、Google、DigitalOcean 等前缀和赋值场景。保留本项目 S 对 20 字符以上 `sk-` 值及 `_`、`-` 的支持。

执行时先用关键词筛选规则，再按实际捕获组的起止位置提取凭据。规则原有的熵阈值及正则允许项继续生效。停用词只忽略整个值相等的情况，不会因为真实凭据中恰好包含 `example` 就跳过它。此处移植的是签名集，不是 Gitleaks 的仓库扫描器、路径策略或完整配置系统。

开启 G 后，明确的凭据字段会整体替换非空字符串，包括 `api_key`、`api_token`、`access_key`、`access_key_id`、`access_key_secret`、`secret_access_key`、`access_token`、`refresh_token`、`client_secret`、`password`、`passwd`、`secret`、`token`、`authorization`、`private_key`、`credential`、`credentials`。匹配忽略大小写及下划线、连字符、点和空格；数组中的字符串继承所在字段名。字段名称精确匹配，通用 `name`、`key` 不会因此整值替换。停用 G 后，这层字段策略也停用。

已有 GitHub、GitLab、AWS、JWT、私钥等标签继续使用，其余服务凭据显示为 `CREDENTIAL`。连接串沿用本项目较保守的整串检测，包括不含用户名或密码的连接 URL。

## JSON 字段与协议

通用 JSON 的 `id`、`name`、`type`、`model`、`format` 和包含 `image_url` 的业务字段不再全局豁免。仅在识别的 API 协议及真实结构位置保留必要值：

| 位置 | 保留内容 |
|---|---|
| Chat / Responses / Anthropic 的协议字段 | 模型名、消息角色、工具名及调用标识符 |
| 实际多模态块 | 图像 URL、文件载荷及文件 ID、音频载荷；Anthropic base64 / URL source |
| Anthropic assistant 历史消息 | 整个 `thinking` / `redacted_thinking` 块 |
| Chat assistant 历史消息 | `reasoning`、`reasoning_content`、`reasoning_details` |
| Responses 历史状态 | 顶层 `previous_response_id`，`input` 中的 `reasoning` / `compaction` 项 |
| 工具及输出 schema | 类型、格式、引用、正则及 required / dependentRequired 的属性名称 |

用户或工具数据中的同名 reasoning / signature 字段继续扫描。Responses 中明确标为用户角色的 reasoning 项也不作为历史状态豁免。媒体描述、文件名、文档标题、Anthropic 文本 document source，以及 schema 的 description、examples、default、enum、const 继续扫描。普通对象里拼写成 `[0]` 的键不会被当作协议数组索引。

JSON 对象键维持原样，以免改变参数结构；键本身命中规则时，仅在日志和错误的字段路径中显示 `<redacted-key>`。网关的处理范围是值，不能借此隐藏存放在键中的敏感信息。

## 字符串内嵌 JSON

所有字符串字段中，以 `{` 或 `[` 开头的完整 JSON 会递归检测，`arguments` 和 `partial_json` 另外支持任意完整 JSON 值。先解码字符串值再匹配，可发现 `alice\u0040example.com`；内部也支持凭据字段和数字校验，不继承外层协议豁免。无法解析为 JSON 的普通文本继续按文本规则处理。

只改写被替换的字符串值；对象键、空白、数字字面量和未触及的转义保持原字节。还原完整 JSON 文本时逐层转义，原值中的引号、换行、反斜杠不会破坏 JSON。工具参数 SSE 分片继续沿用 JSON 字符串转义策略，普通文本分片不被一律当作工具 JSON。

结构深度预算为 64 层，包含外层 JSON 与内嵌文档的结构；连续字符串编码的 JSON 文档最多 8 层。超限返回 HTTP 413、错误码 `json_nesting_limit`，不会回退到明文转发。日志命中字段只记录最外层字符串的路径，例如 `$.messages[0].content`。

## 数字类型的敏感值

启用 P / I / B 时，匹配相应规则的 JSON 数字会阻断整个请求，返回 HTTP 400：

```json
{
  "error": {
    "type": "redact_gateway_error",
    "code": "numeric_sensitive_value",
    "message": "sensitive numeric value: PHONE at $.data.phone; send this value as a JSON string"
  }
}
```

调用方应把 `{"phone":13800138000}` 改为 `{"phone":"13800138000"}`，随后按普通字符串脱敏及还原。错误只显示规则类型和安全字段路径，不含原始数字。

检查十进制数值的精确整数形式，`13800138000.0`、`1.3800138e10` 同样会被阻断。JSON 解码使用 `UseNumber`，归一化不依赖浮点数，也不会为巨大指数分配巨大整数。未命中规则的数字、负数、非整数和未启用的检测类别维持原类型及数值。网关集成测试同时验证拒绝状态、零上游请求及字符串版本的脱敏转发与还原。

P 的国际号码验证完整的 8–15 位数字和边界，支持常见空格、括号及连字符分组；不会截取超长号码的一部分。B 要求 13–19 位、非重复单一数字且通过 Luhn。I 在校验位之外验证真实出生日期、非零顺序码及宽松的省级地区前缀，不依赖容易过时的县级行政区表。

## 数据来源和更新

导入来源：[Cosy Redact Gateway](https://github.com/CassiopeiaCode/CosyRedactGateway)，固定版本 `0e2be2e3ba7fcbe982942a941e3a2fd1b89e87f6`。模型、阈值和校准词表来自该版本；凭据签名的上游来源包括 Gitleaks 默认规则。

```powershell
node scripts/import-detector-data.mjs D:/Projects/CosyRedactGateway
go test ./internal/redact ./internal/gateway
```

导入脚本检查完整提交号及源文件是否修改，生成 `internal/redact/data/*.json`、`internal/redact/testdata/entropy-words.json` 和第三方许可文件。运行时通过 Go embed 使用固定数据，不依赖 Node 或网络。升级源版本需先审查模型、规则和许可变化，再修改导入器固定版本并重新验证。

本项目将模型计算重写为 Go 固定查表，将正则标志和 Unicode 转义转换为 RE2 格式，并实现上述区间合并、精确捕获、整值停用词、字段及数字策略。生成数据的元信息也记录来源、版本与改动说明。

Cosy 的 Apache-2.0 许可、NOTICE 及 Gitleaks MIT 声明保存在 [third_party/cosy-redact-gateway](../third_party/cosy-redact-gateway/)。Docker 镜像包含 `/usr/share/licenses/redact-gateway/third_party` 及本说明 `/usr/share/doc/redact-gateway/detection.md`；单独分发编译后的程序时也应附上这些许可和说明。
