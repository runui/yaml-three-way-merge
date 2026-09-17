# YAML Three-Way Merge Validation

这个项目独立验证 ZimaOS App Management 当前的 Compose 三方合并实现。它不复制生产代码，而是通过本地 `replace` 直接调用当前工作区中的：

- `override.Build`
- `compose.MergeYAML`
- `override.RebaseRepositoryUpdate`
- `structured-merge-diff/v7` 的 `Compare`、`RemoveItems`、`ExtractItems` 和 `Merge`

## 合并原则

输入有三个状态：

```text
B = base_old.yaml
U = 用户基于 B 得到的完整有效配置
N = base_new.yaml
```

对于具有稳定身份的逻辑配置项，目标规则是：

```text
如果 U 相对 B 有修改、新增或删除：使用 U
如果 U 与 B 相同：跟随 N
```

用户优先作用于最小可识别单元：

- scalar：字段路径
- map：map key
- 有稳定身份的数组：数组元素 identity
- 无法安全识别 identity 的数组：整体回退，并报告风险

## 验证链路

每个案例执行真实生产链路：

```text
old override = Build(B, U)
reconstructed U = MergeYAML(B, old override)
new override = RebaseRepositoryUpdate(B, N, old override)
actual = MergeYAML(N, new override)
```

随后检查：

- `Build + MergeYAML` 是否还原用户完整配置
- 当前三方合并结果是否符合目标用户优先语义
- 生成的新 override 是否幂等
- 已知差异是否仍被稳定归类

## 运行

项目位于嵌套 Go module 中，必须关闭父目录的 `go.work`：

```bash
GOWORK=off go test -count=1 ./...
GOWORK=off go run ./cmd/validate
```

输出 JSON：

```bash
GOWORK=off go run ./cmd/validate -json
```

只要存在不符合目标语义的案例就返回非零状态：

```bash
GOWORK=off go run ./cmd/validate -strict
```

矩阵本身是逻辑基线测试集合，不是性能 benchmark。运行指定基线：

```bash
GOWORK=off go test -count=1 -run TestMergeBaselineMatrix ./internal/validation
```

每个逻辑案例下分别执行 `current` 和 `structured-merge-diff` 两个子测试。当前实现允许明确登记的 `KNOWN_GAP` 保持通过；SMD 试验实现默认必须符合目标语义，只有无法安全定义合并规则的字段才单独登记差异。

## SMD 试验实现

SMD adapter 先将 Compose 的 map/list、短/长语法标准化，再使用 schema-aware field paths 计算 `base_old -> user` 的增删改，将这些用户变化应用到 `base_new`。

端口会注入仅存在于合并过程中的 `__merge_id`。它先按完整 Compose identity 匹配，再仅在 `host_ip + target + protocol` 在相关状态中唯一时关联 published port 的变化；遇到多绑定歧义会返回错误，不会猜测。该内部字段在输出前删除。

## 当前覆盖

第一批案例覆盖：

- 具有稳定身份配置项的完整 15 状态矩阵
- scalar 和 service map 的标准用户优先规则
- `environment`
- `ports`
- `volumes`
- `devices`
- `configs`
- `labels` 的 map/list 表示
- `depends_on` 的短语法
- `env_file`
- `cap_add`
- `command`
- 未知 `x-*` 数组

其中包含当前已知的关键差异：

- 端口 published 值变化后，当前 identity 不能关联旧项，用户删除可能被远程修改复活
- 用户和远程同时修改同一 container port 时，当前实现可能同时保留两项
- `devices`、`depends_on`、`env_file`、集合数组和未知数组在三方合并时按整个数组回退，可能丢失远程独立新增项

后续应继续扩展 `Cases()`，但不要在本项目中复制或修补生产合并算法。
