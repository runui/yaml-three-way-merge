package validation

type Expectation string

const (
	ExpectationMatch    Expectation = "MATCH"
	ExpectationKnownGap Expectation = "KNOWN_GAP"
)

type Case struct {
	ID             string      `json:"id"`
	Category       string      `json:"category"`
	Field          string      `json:"field"`
	Identity       string      `json:"identity"`
	Description    string      `json:"description"`
	BaseOld        string      `json:"base_old"`
	User           string      `json:"user"`
	BaseNew        string      `json:"base_new"`
	Expected       string      `json:"expected"`
	Expectation    Expectation `json:"expectation"`
	SMDExpectation Expectation `json:"smd_expectation,omitempty"`
}

func Cases() []Case {
	cases := []Case{
		{
			ID: "scalar-user-untouched-follows-remote", Category: "scalar", Field: "services.app.restart", Identity: "field path",
			Description: "用户没有修改 scalar，结果跟随远程修改",
			BaseOld:     compose("    restart: always\n"),
			User:        compose("    restart: always\n"),
			BaseNew:     compose("    restart: unless-stopped\n"),
			Expected:    compose("    restart: unless-stopped\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "scalar-user-change-wins", Category: "scalar", Field: "services.app.restart", Identity: "field path",
			Description: "用户和远程同时修改 scalar，用户值优先",
			BaseOld:     compose("    restart: always\n"),
			User:        compose("    restart: no\n"),
			BaseNew:     compose("    restart: unless-stopped\n"),
			Expected:    compose("    restart: no\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "environment-user-delete-and-remote-add", Category: "key-value", Field: "services.app.environment", Identity: "environment key",
			Description: "用户删除旧变量，远程新增的独立变量仍然进入结果",
			BaseOld:     compose("    environment:\n      KEEP: old\n      DELETE_ME: old\n"),
			User:        compose("    environment:\n      KEEP: user\n"),
			BaseNew:     compose("    environment:\n      KEEP: remote\n      DELETE_ME: remote\n      REMOTE_ADD: remote\n"),
			Expected:    compose("    environment:\n      KEEP: user\n      REMOTE_ADD: remote\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "volume-user-change-and-remote-add", Category: "unique-resource", Field: "services.app.volumes", Identity: "container target",
			Description: "用户修改同 target 的 source，同时保留远程新增 target",
			BaseOld:     compose("    volumes:\n      - /base:/data\n"),
			User:        compose("    volumes:\n      - /user:/data\n"),
			BaseNew:     compose("    volumes:\n      - /base:/data\n      - /remote:/config\n"),
			Expected:    compose("    volumes:\n      - /user:/data\n      - /remote:/config\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "port-user-change-and-remote-add", Category: "unique-resource", Field: "services.app.ports", Identity: "target + protocol (desired)",
			Description: "用户修改 published port，同时保留远程新增的其他 container port",
			BaseOld:     compose("    ports:\n      - 8080:80\n"),
			User:        compose("    ports:\n      - 8888:80\n"),
			BaseNew:     compose("    ports:\n      - 8080:80\n      - 9090:90\n"),
			Expected:    compose("    ports:\n      - 8888:80\n      - 9090:90\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "port-user-delete-remote-changes-published", Category: "unique-resource", Field: "services.app.ports", Identity: "target + protocol (desired)",
			Description: "用户删除旧 container port 后，远程修改 published port 不应将其复活",
			BaseOld:     compose("    ports:\n      - 8081:81\n"),
			User:        compose(""),
			BaseNew:     compose("    ports:\n      - 9091:81\n"),
			Expected:    compose(""), Expectation: ExpectationKnownGap,
		},
		{
			ID: "port-both-change-same-container-port", Category: "unique-resource", Field: "services.app.ports", Identity: "target + protocol (desired)",
			Description: "用户和远程修改同一 container port，结果只保留用户映射",
			BaseOld:     compose("    ports:\n      - 8080:80\n"),
			User:        compose("    ports:\n      - 8888:80\n"),
			BaseNew:     compose("    ports:\n      - 9999:80\n"),
			Expected:    compose("    ports:\n      - 8888:80\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "config-user-change-and-remote-add", Category: "unique-resource", Field: "services.app.configs", Identity: "target, otherwise source",
			Description: "用户修改同 target 的 config source，同时保留远程新增 target",
			BaseOld:     withTopLevel(compose("    configs:\n      - source: old_config\n        target: /config.json\n"), "configs:\n  old_config: {}\n  user_config: {}\n  remote_config: {}\n"),
			User:        withTopLevel(compose("    configs:\n      - source: user_config\n        target: /config.json\n"), "configs:\n  old_config: {}\n  user_config: {}\n  remote_config: {}\n"),
			BaseNew:     withTopLevel(compose("    configs:\n      - source: old_config\n        target: /config.json\n      - source: remote_config\n        target: /remote.json\n"), "configs:\n  old_config: {}\n  user_config: {}\n  remote_config: {}\n"),
			Expected:    withTopLevel(compose("    configs:\n      - source: user_config\n        target: /config.json\n      - source: remote_config\n        target: /remote.json\n"), "configs:\n  old_config: {}\n  user_config: {}\n  remote_config: {}\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "device-user-change-and-remote-add", Category: "merge-rebase-asymmetry", Field: "services.app.devices", Identity: "container device target",
			Description: "二方 merge 支持 device target，但三方 rebase 当前整体回退并丢失远程新增项",
			BaseOld:     compose("    devices:\n      - /dev/video0:/dev/video0\n"),
			User:        compose("    devices:\n      - /dev/custom:/dev/video0\n"),
			BaseNew:     compose("    devices:\n      - /dev/video0:/dev/video0\n      - /dev/dri:/dev/dri\n"),
			Expected:    compose("    devices:\n      - /dev/custom:/dev/video0\n      - /dev/dri:/dev/dri\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "labels-list-user-change-and-remote-add", Category: "map-or-list", Field: "services.app.labels", Identity: "label key",
			Description: "labels 使用 list 语法时也应逐 key 合并用户修改和远程新增",
			BaseOld:     compose("    labels:\n      - mode=old\n"),
			User:        compose("    labels:\n      - mode=user\n"),
			BaseNew:     compose("    labels:\n      - mode=remote\n      - remote.added=true\n"),
			Expected:    compose("    labels:\n      mode: user\n      remote.added: \"true\"\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "depends-on-user-add-and-remote-add", Category: "name-map-or-list", Field: "services.app.depends_on", Identity: "service name",
			Description: "用户和远程分别增加依赖服务时应同时保留",
			BaseOld:     composeWithServices("    depends_on:\n      - db\n", "  db:\n    image: busybox\n  user-worker:\n    image: busybox\n  remote-worker:\n    image: busybox\n"),
			User:        composeWithServices("    depends_on:\n      - db\n      - user-worker\n", "  db:\n    image: busybox\n  user-worker:\n    image: busybox\n  remote-worker:\n    image: busybox\n"),
			BaseNew:     composeWithServices("    depends_on:\n      - db\n      - remote-worker\n", "  db:\n    image: busybox\n  user-worker:\n    image: busybox\n  remote-worker:\n    image: busybox\n"),
			Expected:    composeWithServices("    depends_on:\n      db:\n        condition: service_started\n        required: true\n      remote-worker:\n        condition: service_started\n        required: true\n      user-worker:\n        condition: service_started\n        required: true\n", "  db:\n    image: busybox\n  user-worker:\n    image: busybox\n  remote-worker:\n    image: busybox\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "env-file-user-add-and-remote-add", Category: "scalar-or-list", Field: "services.app.env_file", Identity: "file path",
			Description: "用户和远程分别增加 env file 时应同时保留",
			BaseOld:     compose("    env_file:\n      - .env\n"),
			User:        compose("    env_file:\n      - .env\n      - user.env\n"),
			BaseNew:     compose("    env_file:\n      - .env\n      - remote.env\n"),
			Expected:    compose("    env_file:\n      - .env\n      - remote.env\n      - user.env\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "cap-add-user-add-and-remote-add", Category: "set-like-list", Field: "services.app.cap_add", Identity: "capability name",
			Description: "用户和远程分别增加 capability 时应同时保留",
			BaseOld:     compose("    cap_add:\n      - NET_ADMIN\n"),
			User:        compose("    cap_add:\n      - NET_ADMIN\n      - SYS_ADMIN\n"),
			BaseNew:     compose("    cap_add:\n      - NET_ADMIN\n      - CHOWN\n"),
			Expected:    compose("    cap_add:\n      - NET_ADMIN\n      - CHOWN\n      - SYS_ADMIN\n"), Expectation: ExpectationKnownGap,
		},
		{
			ID: "command-conflict-user-wins-atomically", Category: "replace-only", Field: "services.app.command", Identity: "whole field",
			Description: "command 是整体值，双方冲突时保留完整用户命令，不能按参数拼接",
			BaseOld:     compose("    command: [server, --port, \"80\"]\n"),
			User:        compose("    command: [server, --port, \"8080\"]\n"),
			BaseNew:     compose("    command: [server, --port, \"80\", --verbose]\n"),
			Expected:    compose("    command: [server, --port, \"8080\"]\n"), Expectation: ExpectationMatch,
		},
		{
			ID: "extension-list-user-add-and-remote-add", Category: "generic-list", Field: "x-example.routes", Identity: "undefined",
			Description: "未知数组没有稳定 identity，当前整体用户优先会丢失远程新增项",
			BaseOld:     withTopLevel(compose(""), "x-example:\n  routes:\n    - /base\n"),
			User:        withTopLevel(compose(""), "x-example:\n  routes:\n    - /base\n    - /user\n"),
			BaseNew:     withTopLevel(compose(""), "x-example:\n  routes:\n    - /base\n    - /remote\n"),
			Expected:    withTopLevel(compose(""), "x-example:\n  routes:\n    - /base\n    - /remote\n    - /user\n"), Expectation: ExpectationKnownGap, SMDExpectation: ExpectationKnownGap,
		},
	}
	return append(stableItemStateMatrix(), cases...)
}

func (c Case) ExpectedSMDClassification() Expectation {
	if c.SMDExpectation == "" {
		return ExpectationMatch
	}
	return c.SMDExpectation
}

type itemState struct {
	present bool
	value   string
}

func stableItemStateMatrix() []Case {
	absent := itemState{}
	a := itemState{present: true, value: "a"}
	u := itemState{present: true, value: "user"}
	r := itemState{present: true, value: "remote"}
	tests := []struct {
		id          string
		description string
		baseOld     itemState
		user        itemState
		baseNew     itemState
		expected    itemState
	}{
		{id: "absent-unchanged", description: "三方都不存在", baseOld: absent, user: absent, baseNew: absent, expected: absent},
		{id: "remote-add", description: "用户未干预，接受远程新增", baseOld: absent, user: absent, baseNew: r, expected: r},
		{id: "user-add", description: "保留用户新增", baseOld: absent, user: u, baseNew: absent, expected: u},
		{id: "both-add-same", description: "双方新增相同值，只保留一份", baseOld: absent, user: u, baseNew: u, expected: u},
		{id: "both-add-different", description: "双方新增冲突，用户值优先", baseOld: absent, user: u, baseNew: r, expected: u},
		{id: "present-unchanged", description: "三方都未修改", baseOld: a, user: a, baseNew: a, expected: a},
		{id: "remote-modify", description: "用户未修改，接受远程修改", baseOld: a, user: a, baseNew: r, expected: r},
		{id: "remote-delete", description: "用户未修改，接受远程删除", baseOld: a, user: a, baseNew: absent, expected: absent},
		{id: "user-modify", description: "用户修改优先", baseOld: a, user: u, baseNew: a, expected: u},
		{id: "user-delete", description: "用户删除优先", baseOld: a, user: absent, baseNew: a, expected: absent},
		{id: "both-modify-same", description: "双方修改为相同值", baseOld: a, user: u, baseNew: u, expected: u},
		{id: "both-modify-different", description: "双方修改冲突，用户值优先", baseOld: a, user: u, baseNew: r, expected: u},
		{id: "user-modify-remote-delete", description: "远程删除，用户修改优先", baseOld: a, user: u, baseNew: absent, expected: u},
		{id: "user-delete-remote-modify", description: "用户删除优先，远程修改不能复活配置项", baseOld: a, user: absent, baseNew: r, expected: absent},
		{id: "both-delete", description: "双方都删除", baseOld: a, user: absent, baseNew: absent, expected: absent},
	}

	cases := make([]Case, 0, len(tests))
	for _, test := range tests {
		cases = append(cases, Case{
			ID:          "stable-item-" + test.id,
			Category:    "stable-item-state-matrix",
			Field:       "x-validation.item",
			Identity:    "mapping key",
			Description: test.description,
			BaseOld:     stableItemCompose(test.baseOld),
			User:        stableItemCompose(test.user),
			BaseNew:     stableItemCompose(test.baseNew),
			Expected:    stableItemCompose(test.expected),
			Expectation: ExpectationMatch,
		})
	}
	return cases
}

func stableItemCompose(state itemState) string {
	if !state.present {
		return compose("")
	}
	return withTopLevel(compose(""), "x-validation:\n  item: "+state.value+"\n")
}

func compose(serviceFields string) string {
	return "services:\n  app:\n    image: busybox\n" + serviceFields
}

func composeWithServices(serviceFields, services string) string {
	return compose(serviceFields) + services
}

func withTopLevel(composeYAML, topLevel string) string {
	return composeYAML + topLevel
}
