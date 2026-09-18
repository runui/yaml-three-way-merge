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

对于每一个逻辑数组元素：

```text
user 与 oldbase 相同：跟随 newbase
user 相对 oldbase 修改：使用 user
user 相对 oldbase 删除：保持删除
user 相对 oldbase 新增：保留用户新增
newbase 新增且用户没有操作：继承远程新增
```

规则作用于逻辑元素，不是整个数组。测试数据中的期望结果独立生成，不调用生产合并算法。

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

`user.yml` 是真实覆盖层，删除数组时使用 `!reset []`；删除后写入用户值时使用两个 YAML document 表达 reset 和新值。

## 穷举范围

- 70 个数组、嵌套数组或数组短语法字段。
- 每个字段包含 15 个单逻辑项三方状态。
- 除整体原子字段外，每个字段包含两个逻辑项的 `15 × 15 = 225` 完整组合。
- `ports` 额外包含用户提供的 7 个固定验收场景。
- 当前共 16,132 个 fixture case，每个 case 有 5 个 YAML 文件。

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

当前实现不能通过全部用户期望是正常现象；失败 case 就是后续生产合并算法的修复基准。

只输出汇总：

```bash
GOWORK=off go run ./cmd/validate -fixtures fixtures -json
```

当前基线：

```text
total:          16132
matched:        12243
mismatched:      3889
errors:             0
non-idempotent:     0
```
