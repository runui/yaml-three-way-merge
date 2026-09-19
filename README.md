# YAML Three-Way Merge Validation

该项目以用户意图为唯一基准，验证 ZimaOS App Management 的 YAML 三方合并：

```text
oldbase.yml  当前应用商店版本
user.yml     用户相对于 oldbase 的真实覆盖层
newbase.yml  应用商店待升级版本
expected.yml 用户期望的最终有效配置
```

当前阶段覆盖 Compose v1.20.2 与 x-casaos 的全部字段：YAML 原生 sequence、支持 sequence 短语法的字段、标量叶子、固定结构 mapping 的每个叶子路径，以及自由 key/value mapping。数组类字段按逻辑元素三方合并，标量字段作为整体值合并。

## 用户意图规则

对于声明为 `logical-items` 的字段，每一个逻辑数组元素遵循：

```text
user 与 oldbase 相同：跟随 newbase
user 相对 oldbase 修改：使用 user
user 相对 oldbase 删除：保持删除
user 相对 oldbase 新增：保留用户新增
newbase 新增且用户没有操作：继承远程新增
```

普通 `ordered-list` 字段没有稳定的跨版本元素 identity，将完整列表视为一个值：user 列表未变更时跟随 newbase，否则使用完整 user 列表。pair 中的两个状态只用于构造列表片段，不代表可独立三方合并的元素。

标量字段（`class: atomic`）没有内部 identity，按整体值处理：user 未变更时跟随 newbase，否则使用 user，用户删除保持删除。`build`、`extends` 这类同时接受 scalar 和 mapping 的字段在 scalar 形态下按原子值建模，其 mapping 形态按每个叶子路径单独建模。

自由 key/value mapping（`logging.options`、`driver_opts`、`ulimits`、`x-casaos.title` 等）按 key 为逻辑项建模；每个 key 是一个逻辑项，其 value 是该逻辑项的值。

测试数据中的期望结果独立生成，不调用生产合并算法。fixture 生成器只依据 states.go 的用户意图矩阵计算 expected，不会因为某个算法失败而改写期望。

## 数据布局

所有测试数据均以文件夹和 YAML 文件保存：

```text
fixtures/
  manifest.yml
  <merge-class>/
    <field>/
      single/
        <state>/
          case.yml
          oldbase.yml
          user.yml
          newbase.yml
          expected.yml
      pair/
        <state-a>__<state-b>/
          case.yml
          oldbase.yml
          user.yml
          newbase.yml
          expected.yml
```

`user.yml` 是真实覆盖层。direct sequence 只在目标字段使用 `!reset []`；wrapped sequence 在最近可完整重建的 owner sequence 使用 reset。删除后写入用户值时使用两个 YAML document 表达 reset 和新值，不会重置无关的 `services` 或其他顶层资源。

## 语法变体

同一个逻辑字段可以有多种语法正确但形状不同的 Compose 写法。生成器把它们建模为一等维度：

- `internal/corpus/variants_*.go` 通过 `registerVariants(fieldID, FieldVariant{...})` 注册变体；每个变体提供 `Render`（把逻辑值编码成该语法）和可选的 `ResetBoundary`。
- `ArrayFields()` 把基础字段与已注册变体展开为独立字段条目，ID 形如 `<fieldID>@<variant>`；语义（`pair_semantics`）保持不变。
- 变体文件彼此独立，可以按字段分组并行维护。

已覆盖的变体分组：

- `variants_ports.go`：`service.ports@short`（`"published:target"`）。
    - `variants_maps.go`：14 个键值字段的 `@map`（`["k=v"]` → `{k: v}`）。
- `variants_resources.go`：`service.volumes@short`（`"source:target"`）。`service.devices` 在固定的 Compose 版本（compose-go v1.20.2）中只接受字符串元素，长写法 mapping 不是合法 Compose，因此不注册 `@long` 变体。
- `variants_scalars.go`：`service.command@string`、`service.entrypoint@string`、`healthcheck.test@string`、`service.env-file@scalar`、`service.tmpfs@scalar`、`service.dns@scalar`、`service.dns-search@scalar`。
- `variants_named_maps.go`：`service.depends-on@map`、`service.networks@map`。

变体只改变语法，不改变逻辑项身份模型。像 configs/secrets 的纯短名写法会同时充当 source 和 target，无法把「稳定身份」和「被修改的值」分开表达，因此不作为变体。

## 穷举范围

- 259 个基础字段条目（66 个数组类字段 + 193 个标量/mapping 字段），展开后共 284 个字段条目（含 25 个语法变体）。
- 每个字段条目包含 15 个单逻辑项三方状态。
- 除整体原子字段外，每个字段条目包含两个逻辑项的 `15 × 15 = 225` 完整组合。
- 每个数组类字段使用第三个逻辑项生成 6 个 multi-item 场景，覆盖「多个元素中只改其中一个/多个」：
  - `01-user-modifies-first-remote-modifies-second`
  - `02-user-changes-first-and-third-remote-changes-second`
  - `03-user-deletes-first-remote-modifies-second`
  - `04-user-modifies-first-remote-deletes-second`
  - `05-user-modifies-second-remote-changes-all`
  - `06-both-add-third-element-user-wins`
- 多资源场景（`multi/`）覆盖「多个 service / network / volume / config / secret 中只改其中一个」：每个字段条目 4 个场景，其中 `app` 与 sibling 资源各自独立变化，验证合并 scope 不会泄漏到未改动的资源。
- 多属性场景（`multi-attribute/`）覆盖「数组元素内部多个属性只改其中一个/多个」：8 个字段条目各 4 个场景。
- `ports` 额外包含用户提供的 7 个固定验收场景。
- 当前共 28,630 个 fixture case，其中 1,645 个为上述显式场景（`explicit_cases`），其余为状态矩阵，每个 case 有 5 个 YAML 文件。
- manifest 记录每个字段条目的 pair semantics：`logical-items`、`whole-list` 或 `none`。

字段和数量记录在 `fixtures/manifest.yml`。完整性元测试会检查：

- 字段条目 ID 唯一，基础字段不重复路径；
- 每个字段条目恰好有 15 个单项状态；
- 每个非原子字段条目恰好有 225 个双项组合；
- 磁盘 case 数与 manifest 一致，且显式场景数与 `explicit_cases` 一致；
- 每个 case 的 5 个 YAML 文件都存在；
- 每个 case 的 `user.yml` 能通过 Compose merge 复现声明的用户有效状态；
- 每个 `oldbase.yml` / `newbase.yml` / `expected.yml` 都通过固定的 Compose JSON Schema 校验（`TestFixtureDocumentsConformToComposeSchema`），保证 fixture 是用户真实可写的 Compose；
- 不存在「三份输入结构相同但 expected 不同」的不可判定 case（`network.ipam.config` 与 network aliases/link-local-ips 的 50 个历史 case 除外，见下文）。

fixture 是字段级的可合并片段，不是完整可安装项目：`configs`/`secrets`/`networks`/`volumes`/`depends_on` 等引用目标通常不在片段内声明，`env_file`/`include` 的路径也是符号化的。这保证每个 case 只隔离被测字段，同时不承诺完整项目加载。

“穷举”指穷举用户意图的行为状态组合，不可能穷举无限的字符串、端口号或数组长度，也不可能穷举所有 YAML 序列化风格（flow/block、锚点等）。多资源与多属性场景是按行为类别精选的，不是笛卡尔积。

## 生成数据

```bash
GOWORK=off go run ./cmd/generate-fixtures -output fixtures
```

生成器会重建整个 `fixtures` 目录。运行时测试只读取落盘 YAML，不动态隐藏测试数据。

## 验证

检查 fixture 完整性：

```bash
GOWORK=off go test -run 'TestFixtureManifestIsComplete|TestArrayFieldRegistryIsAuditable' ./internal/validation
```

运行双行为 intent 引擎的回归门禁：

```bash
GOWORK=off go test -run TestFixtureCorpusAgainstIntentImplementation ./internal/validation
```

该门禁排除 50 个不可判定 case，并把其余不匹配按字段与 `knownAlgorithmBoundary` 逐字段对比。这是一个双向 ratchet：某字段超过预算会失败（出现新边界），低于预算也会失败（修好了边界必须同步收紧预算）。fixture 永远按用户意图生成，不因算法失败而调整。

严格运行生产实现的全部用户期望测试：

```bash
GOWORK=off go test -run TestFixtureCorpusAgainstCurrentImplementation ./internal/validation
```

当前生产实现不能通过全部用户期望是正常现象；失败 case 是后续生产合并算法的修复基准。fixture 生成器会单独验证覆盖层能复现声明的用户有效状态，并审计 reset scope。

只输出汇总：

```bash
GOWORK=off go run ./cmd/validate -fixtures fixtures -json
```

使用 structured-merge-diff 直接编译多文档 `user.yml` 中的 writes 和 `!reset` removals，并与项目算法对比：

```bash
GOWORK=off go run ./cmd/validate-smd -fixtures fixtures -compare-project -json
```

该路径在 SMD typed value 内部通过 `RemoveItems` 和 partial `Merge` 应用用户覆盖层，不会先用 Compose merge 还原完整 user YAML。

使用独立的双行为 SMD 策略：分别计算并 replay `oldbase -> user prediction` 与 `oldbase -> newbase`，再把用户行为应用到上游行为预期：

```bash
GOWORK=off go run ./cmd/validate-smd-intent -fixtures fixtures -json
```

三个命令分别只运行各自算法：

- `cmd/validate`：项目 `RebaseRepositoryUpdate`；
- `cmd/validate-smd`：直接将用户 delta 应用到 newbase 的 SMD 策略；
- `cmd/validate-smd-intent`：显式双 delta、双 replay 和冲突报告的 SMD 策略。

当前双行为 SMD 基线：

```text
total:          28630
matched:        27430
mismatched:      1200
errors:             0
```

直接 SMD 基线（`cmd/validate-smd` 不带 `-compare-project`，仅直接应用用户 delta）：

```text
total:          28630
matched:        26559
mismatched:      2071
errors:             0
```

三个命令各自带有独立的回归测试，分别运行：

```bash
GOWORK=off go test -count=1 ./cmd/validate
GOWORK=off go test -count=1 ./cmd/validate-smd
GOWORK=off go test -count=1 ./cmd/validate-smd-intent
```

每个 `cmd/*/main_test.go` 会在全量 fixture 上运行该命令并锁定上面的对应基线；算法行为改变时需同步更新测试常量与本节基线。

双行为策略的关键语义：

- **逻辑项身份**：对声明为 `logical-items` 的集合，先按结构键（`target`/`path`/端口元组等）关联，再按稳定的 origin token（最后一个 `-` 后的后缀）关联；同 token 的两个新增视为同一逻辑项的修改（user 胜），不同 token 视为独立新增。无法唯一关联时不按位置或残余数量猜测。
- **scope 精确的 removal**：用户删除时，把嵌套 removal 提升到最近的不再存在的 keyed item（例如删除某个 volume 则移除该 item，清空嵌套 set 只移除 set 元素），并丢弃冗余父 removal，避免误删上游在同一父容器下新增的 sibling；保留下来的 item 会从 user prediction 回写，避免部分写入丢失未变更字段。
- **空容器等价**：空 list/map 与缺失等价，保证 replay/比较的表示一致。
- **schema 完整**：deploy devices、nested capabilities/device_ids、network ipam.config、service network aliases/link_local_ips 均按逻辑项建模，而非退化为原子值。
- **ports**：以 `host_ip+target+protocol` 作为列表 key，部分写入仍保留身份字段。
- **引用身份**：configs/secrets 显式 `target` 稳定时以 target 为身份，短写法（`source==target`）回退到 origin token。

### 已知 50 个不可判定 case

`TestFixtureCorpusAgainstIntentImplementation` 通过 `knownUnsupportedIntent` 显式排除 50 个 case：

- 42 个 `network.ipam.config`：原写法只写 `subnet`，而 `subnet` 本身就是被修改的值。`05-both-add-different`（期望 user 胜）与 `03-user-add__02-remote-add`（期望并集）在三份输入上结构同构、只差值本身，任何只看输入文档的算法都无法区分。
- 8 个 `service.network.aliases` / `service.network.link-local-ips`：两端都删除嵌套属性时，期望是「移除整个 network 附着」而不是「保留空附着」，这一策略无法从输入观察出来。

这些 case 保留在语料中作为语法样例，但不作为算法通过标准；修复它们需要修改 fixture 语义或引入输入之外的信息。

当前基线（生产 `RebaseRepositoryUpdate` 尚未统一到上述语义）：

```text
total:          28630
matched:        24210
mismatched:     4420
errors:             0
non-idempotent:     0
```

## 算法边界（fixture 视角）

全量语料从用户意图生成，因此三个实现的残余不匹配就是它们的可观察边界。双行为 intent 引擎的 1,200 个不匹配按根因分为：

- 自由 key/value mapping（`driver_opts`、`ipam.options`、`logging.options`、`storage_opt`、`ulimits`、`x-casaos` 本地化文本、`deploy.labels`、`aux_addresses`、device `options`）：SMD schema 把它们作为 untyped atomic map，改一个 key 会整体替换，而不是按 key 合并。
- `deploy.resources.reservations` 的 `generic_resources` 与 `ports` 的 `host_ip`/`protocol`：被修改的值本身是列表 key 的一部分，修改被观察为 add + remove，而不是同一逻辑项的 modify。
- `deploy.resources.reservations` devices 的 `capabilities`/`device_ids`：嵌套 set 属性在 device item 内被作为整体值。
- volume 的 bind/tmpfs/volume 属性：嵌套 option mapping 在 volume item 内被作为 untyped atomic value。
- `depends_on` 长格式条目：entry mapping 被作为 untyped atomic value，无法按属性合并。
- `service.extends` 的 mapping 形态：整个 extends mapping 被作为 untyped atomic value，`file` 无法按属性合并。

这些计数记录在 `internal/validation/known_boundaries.go`，并由双向 ratchet 强制。
