# YAML 三方合并

[English](README.md) | **简体中文**

用于 Compose YAML 语义三方合并的可组合 Go 库与验证套件。

核心问题是**存在并行定制时的配置演化**：给定旧版上游配置、用户相对于该配置的覆盖层，以及新版上游配置，如何构造一份既保留用户变更、又吸收独立上游变更的有效配置。这要求区分未修改、修改、新增和删除，并通过明确的策略处理双方重叠的变更。

与文本合并不同，语义合并需要同时考虑等价语法表示、字段级合并粒度，以及元素的跨版本身份。mapping 可能表示可独立编辑的键，也可能是原子值；sequence 可能是整体有序值，也可能是具有身份的元素集合。此外，覆盖层表达的是操作而非完整状态，因此字段缺失与显式删除不能作相同解释。当输入不足以确定元素身份或删除范围时，用户期望的结果可能不可唯一判定。

```text
旧基线 + 用户覆盖层 + 新基线 → 最终有效配置
```

公共库将输入编译、变更提取、replay 校验和策略应用拆分为可复用阶段。验证方法以独立生成的用户意图期望为参照，通过有限的行为状态矩阵对比实现，而不从被测算法推导期望结果。套件包含 28,630 条用例，覆盖固定版本的 Compose 模型（`compose-go v1.20.2`）及 `x-casaos` 扩展字段。这是在既定模型内的行为覆盖，不是对任意 YAML 合并正确性的证明，详见[已知边界](#已知边界)。

## 快速开始

### 环境要求

- Go 1.27，与 [go.mod](go.mod) 一致。
- 运行语料测试及验证命令时，需要仓库中的 `fixtures/` 目录。

在仓库根目录运行回归检查：

```bash
GOWORK=off go test -count=1 ./... -skip '^TestFixtureCorpusAgainstCurrentImplementation$'
GOWORK=off go vet ./...
```

被排除的测试用于严格检查现有项目实现是否满足全部用户期望，目前存在已知失败。命令级测试仍会验证该实现已记录的行为基线。

运行 intent 策略并输出报告：

```bash
GOWORK=off go run ./cmd/validate-smd-intent -fixtures fixtures -json
```

两个 SMD 验证命令在存在不匹配或合并错误时返回退出码 `1`，参数、语料加载或报告写入错误时返回 `2`。因此，包含已知不匹配的正常基线报告也会返回非零状态。

## Go 库

导入公共包：

```go
import "github.com/runui/yaml-three-way-merge/merge"
```

### 一次调用完成合并

```go
result, err := merge.Merge(merge.Input{
    PreviousBase: oldYAML,
    UserOverride: overrideYAML,
    TargetBase:   newYAML,
})
if err != nil {
    return err
}
// result.YAML 是最终有效配置。
// result.Report 包含变更、冲突和 replay 校验信息。
```

`UserOverride` 是真实覆盖层，支持多文档 `!reset` 操作，不是已经物化的完整用户配置。

### 按阶段组合

```text
Compile → Scenario ── ApplyDirect() → YAML
              │
              └── Analyze() → Plan ── Report()
                                └── Apply() → Result
```

```go
scenario, err := merge.Compile(input)
if err != nil {
    return err
}
plan, err := scenario.Analyze()
if err != nil {
    return err
}
result, err := plan.Apply()
if err != nil {
    return err
}
```

| API | 职责 |
| --- | --- |
| `Compile(input)` | 归一化语法、关联身份，并在旧基线上编译覆盖层。 |
| `scenario.Analyze()` | 提取双方 delta，验证 replay 能复现各自状态。 |
| `plan.Report()` | 返回应用前的诊断快照。 |
| `plan.Apply()` | 将用户意图应用到上游变更，并处理命名资源的 origin 关联。 |
| `scenario.ApplyDirect()` | 直接应用用户 delta，不进行 intent 删除范围提升或命名 origin 协调。 |
| `Equivalent(left, right)` | 比较建模后的 Compose 语义，而非 YAML 字节。 |

可运行示例见 [merge/example_test.go](merge/example_test.go)。

### API 契约

- 输出是最终有效配置，不是覆盖层；不保留注释、排版和锚点。
- 编译结果绑定本次三份输入：身份关联依赖旧、新基线，不能将 plan 应用到另一份基线。
- `Compile` 返回后可以修改输入缓冲区。输出 YAML 和报告由调用方持有，修改它们不会影响后续调用。
- Scenario 和 Plan 支持顺序复用，暂不承诺同一实例并发调用安全。
- nil 或零值阶段对象返回 `ErrUninitialized`，可通过 `errors.Is` 判断。
- `*merge.Error` 提供 `Stage`（`compile`、`analyze`、`apply` 或 `compare`）并保留底层错误，可通过 `errors.As` 检查；错误文本不是稳定契约。
- 报告路径用于诊断，不是可重放的 patch 格式。`OriginRewrites` 在应用时确定，通过 `Result.Report` 返回。

## 合并语义

期望描述的是用户有效状态相对旧基线的变化：

| 用户操作 | 期望结果 |
| --- | --- |
| 未修改 | 跟随新基线。 |
| 修改 | 保留用户值。 |
| 删除 | 保持删除。 |
| 新增 | 保留用户新增。 |
| 未操作上游新增项 | 继承该新增项。 |

规则按字段模型应用：

- **逻辑项：** 按可识别元素分别合并；自由 key/value mapping 以 key 为逻辑身份。
- **有序列表：** 整个列表作为一个值；用户未修改则跟随上游，否则使用完整用户列表。
- **原子字段：** 按完整值合并，包括 `build`、`extends` 的标量形态；mapping 形态通过叶子路径构造测试。

intent 引擎优先利用可用的结构身份，例如 volume target、path 和端口元组。部分字段使用最后一个 `-` 后的 origin token。身份规则因字段而异：当前模型也包含 IPAM 的位置身份，以及无 token 字符串的位置关联。这些启发式规则并非通用身份保证。

删除处理会保留独立的上游 sibling；当部分写入可能丢失未修改属性时，恢复完整的保留项。语义比较将空 list/map 与缺失容器视为等价。直接策略和 intent 策略的输出路径有意保留不同的空容器清理规则。

## 架构

```text
merge/                       公共 API、输入输出契约、阶段错误
cmd/                         命令参数、策略选择、报告
internal/
  smdmodel/                  共享 typed 模型、schema、归一化、身份关联
  smdmerge/                  直接用户 delta 策略
  smdintent/                 意图分析、replay、精确删除、origin 协调
  compose/                   YAML AST、Compose merge、语义比较
  rebase/                    项目现有的覆盖层 rebase 策略
  corpus/                    字段注册、意图矩阵、fixture 生成与加载
  validation/                用例执行、统计、回归门禁
```

```text
公共 merge API → smdmerge / smdintent → smdmodel
validation → corpus / rebase → compose
```

两套 SMD 策略并列，不相互依赖。共享 typed 操作放入 `smdmodel`，冲突及删除规则放入对应策略包。模型层不依赖 fixture 或验证层。

fixture 期望由 [internal/corpus/states.go](internal/corpus/states.go) 的意图矩阵独立生成，不能使用被测策略生成其期望结果。

## 验证命令

| 命令 | 策略 |
| --- | --- |
| `cmd/validate` | 项目 `RebaseRepositoryUpdate`，包含幂等性检查。 |
| `cmd/validate-smd` | 直接应用 SMD delta。 |
| `cmd/validate-smd-intent` | 通过公共 API 执行 intent 策略，包含双 replay 和冲突报告。 |

```bash
GOWORK=off go run ./cmd/validate -fixtures fixtures -json
GOWORK=off go run ./cmd/validate-smd -fixtures fixtures -json
GOWORK=off go run ./cmd/validate-smd -fixtures fixtures -compare-project -json
GOWORK=off go run ./cmd/validate-smd-intent -fixtures fixtures -json
```

两个 SMD 命令都支持 `-case <case-id>`，用于检查单个 fixture，并在不匹配时输出实际值与期望值。

### 当前基线

| 策略 | 总数 | 匹配 | 不匹配 | 错误 |
| --- | ---: | ---: | ---: | ---: |
| 项目 rebase | 28,630 | 24,210 | 4,420 | 0 |
| 直接 SMD | 28,630 | 27,573 | 1,057 | 0 |
| Intent SMD | 28,630 | 28,541 | 89 | 0 |

项目基线中的非幂等用例数为零。每个命令的 `main_test.go` 锁定全量语料基线；算法行为变化时，需要同步更新测试及中英文表格。

### 专项检查

```bash
# Fixture 完整性与字段注册表
GOWORK=off go test -run 'TestFixtureManifestIsComplete|TestArrayFieldRegistryIsAuditable' ./internal/validation

# Intent 回归门禁
GOWORK=off go test -run TestFixtureCorpusAgainstIntentImplementation ./internal/validation

# 严格检查项目实现的全部用户期望：当前存在已知失败
GOWORK=off go test -run TestFixtureCorpusAgainstCurrentImplementation ./internal/validation
```

intent 门禁排除 50 个不可判定用例，将其余不匹配数量逐字段与 [internal/validation/known_boundaries.go](internal/validation/known_boundaries.go) 对比。门禁采用双向约束：数量增加或减少都会失败，直到更新预算。修复边界后应收紧预算，而不是修改期望来适配算法。

## Fixture 语料

### 布局与覆盖范围

```text
fixtures/
  manifest.yml
  <merge-class>/<field>/single/<state>/
  <merge-class>/<field>/pair/<state-a>__<state-b>/
  multi/
  multi-attribute/
```

每个用例包含五个 YAML 文件：

| 文件 | 含义 |
| --- | --- |
| `case.yml` | 用例元数据。 |
| `oldbase.yml` | 旧版仓库配置。 |
| `user.yml` | 真实用户覆盖层。 |
| `newbase.yml` | 待升级的仓库配置。 |
| `expected.yml` | 独立生成的最终有效配置。 |

语料包含：

- 259 个基础字段条目：66 个数组类、193 个标量/mapping 条目。
- 展开 25 个语法变体后，共 284 个字段条目。
- 每个条目有 15 个单项状态；每个非原子条目有 225 个双项组合。
- 每个数组类条目有 6 个三元素场景；每个字段条目有 4 个多资源场景。
- 8 个字段条目各有 4 个多属性场景，另有 7 个固定端口验收场景。
- 共 28,630 个用例，其中 1,645 个为显式场景。

[manifest](fixtures/manifest.yml) 记录字段数量及 pair semantics（`logical-items`、`whole-list` 或 `none`）。测试检查用例完整性、必要文件、覆盖层声明的用户意图、reset 范围，以及基线和期望文档对固定版本 Compose JSON Schema 的符合性。

fixture 是字段级片段，不是可安装项目：引用的资源可能未声明，文件路径可能是符号化路径。“穷举”指行为状态组合，不是所有取值、列表长度或 YAML 表示形式。

### 语法变体

`internal/corpus/variants_*.go` 注册 `FieldVariant`，提供 `Render` 函数和可选的 `ResetBoundary`。`ArrayFields()` 将其展开为 `<fieldID>@<variant>` 等 ID，同时保持 pair semantics。

变体覆盖 ports/volumes 短写法、14 个 key/value map 形态、command/entrypoint/healthcheck 字符串形态、列表标量形态，以及 `depends_on`/`networks` 的 named-map 形态。固定 Compose 版本不接受 device mapping，因此不注册该变体。configs/secrets 的纯短名写法将 source 与 target 合一，也不作为同一身份模型的语法变体。

### 重新生成

```bash
GOWORK=off go run ./cmd/generate-fixtures -output fixtures
```

该命令会重建整个 `fixtures` 目录。运行时测试读取落盘 YAML。reset 仅作用于目标字段或最近可完整重建的 owner sequence；先删除后写入时使用独立 YAML document，避免重置无关资源。

## 已知边界

### 不可判定的期望

intent 门禁通过 `knownUnsupportedIntent` 显式排除 50 个用例：

- **42 个 IPAM 用例：** `subnet` 既是唯一可观察身份，也是被修改的值。仅凭输入无法确定双方新增代表同一逻辑项还是独立项。
- **8 个 network aliases/link-local-IP 用例：** 期望在删除嵌套属性时移除整个网络附着，但文档本身无法表达这一额外意图。

这些用例仍保留在语料及 CLI 全量统计中。解决它们需要调整 fixture 语义，或提供三份文档之外的信息。

### 可观察的算法限制

intent 策略剩余 89 个不匹配：上述 50 个不可判定用例，以及 39 个待解决边界：

- 10 个身份关联问题：generic-resource kind、端口 `host_ip`/`protocol` 和 IPAM 位置身份。
- 1 个长格式 `depends_on` 修改遇到上游删除条目时，未恢复完整的命名 mapping owner。
- 28 个 device options / IPAM aux_addresses 用例涉及嵌套 key 与 owner 的删除范围或重建范围。部分覆盖层 reset 整个 owner，而期望保留 owner 内独立上游新增 key；不能简单把所有列表项删除改成叶子删除。

已修复的共性问题包括：自由 mapping 按 key 建模、结构 mapping 按属性建模、deploy labels 两种语法归一化、volume 长格式嵌套属性保留，以及嵌套列表写入时恢复最外层缺失 owner。共减少 1,111 个不匹配，未修改 fixture 期望。`ulimits` 按名称合并，每个 limit 值仍作为整体；未声明字段仍保留原子语义。

`known_boundaries.go` 中的逐字段预算是这些限制的可执行记录。后续改进仍以 fixture 的用户期望为准。
