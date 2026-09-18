# YAML Three-Way Merge Validation

该项目以用户意图为唯一基准，验证 ZimaOS App Management 的 YAML 三方合并：

```text
oldbase.yml  当前应用商店版本
user.yml     用户相对于 oldbase 的真实覆盖层
newbase.yml  应用商店待升级版本
expected.yml 用户期望的最终有效配置
```

当前阶段只聚焦数组类字段，包括 YAML 原生 sequence，以及 Compose 中支持 sequence 短语法的字段。

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

测试数据中的期望结果独立生成，不调用生产合并算法。

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

## 穷举范围

- 70 个数组、嵌套数组或数组短语法字段。
- 每个字段包含 15 个单逻辑项三方状态。
- 除整体原子字段外，每个字段包含两个逻辑项的 `15 × 15 = 225` 完整组合。
- `ports` 额外包含用户提供的 7 个固定验收场景。
- 当前共 16,132 个 fixture case，每个 case 有 5 个 YAML 文件。
- manifest 记录每个字段的 pair semantics：`logical-items`、`whole-list` 或 `none`。

字段和数量记录在 `fixtures/manifest.yml`。完整性元测试会检查：

- 字段清单没有重复 ID 或路径；
- 每个字段恰好有 15 个单项状态；
- 每个非原子字段恰好有 225 个双项组合；
- 磁盘 case 数与 manifest 一致；
- 每个 case 的 5 个 YAML 文件都存在。

“穷举”指穷举用户意图的行为状态组合，不可能穷举无限的字符串、端口号或数组长度。

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

严格运行所有用户期望测试：

```bash
GOWORK=off go test -run TestFixtureCorpusAgainstCurrentImplementation ./internal/validation
```

当前实现不能通过全部用户期望是正常现象；失败 case 是后续生产合并算法的修复基准。fixture 生成器会单独验证覆盖层能复现声明的用户有效状态，并审计 reset scope。

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
total:          16132
matched:        14492
mismatched:      1640
errors:             0
```

双行为策略会将 `depends_on` 和 service `networks` 中有唯一稳定 origin token 的 `delete + add` 重建为 modify，再联合解析 user/upstream intent。无法唯一关联时保持原 SMD 行为，不按位置或残余数量猜测。相对直接 SMD 当前新增通过 92 条，未引入已通过 case 回退。

当前基线：

```text
total:          16132
matched:        12993
mismatched:     3139
errors:             0
non-idempotent:     0
```
