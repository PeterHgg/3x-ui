# Clash 订阅模板化核心实现逻辑

## 1. 核心目标
将面板中静态的“Clash 路由规则”文本框升级为 **动态模板引擎**，允许在自定义 YAML 节点组时注入动态变量（如 `{{ .UUID }}`）。

## 2. 核心代码实现

### 第一步：提取客户端 UUID
在 `sub/subClashService.go` 的 `GetClash` 方法中，遍历获取当前订阅链接 (`subId`) 对应的客户端 UUID。
```go
var clientUUID string
for _, inbound := range inbounds {
    for _, client := range clients {
        if client.SubID == subId {
            if clientUUID == "" {
                clientUUID = client.ID 
            }
            // ...
        }
    }
}
```

### 第二步：使用 Go Template 渲染 YAML
在合并规则前，引入 `text/template` 拦截用户在面板输入的 `s.clashRules`，注入上下文变量并进行文本渲染。
```go
if s.enableRouting {
    clashRulesStr := s.clashRules
    
    // 使用 Go 原生 template 引擎解析面板文本
    tmpl, err := template.New("clash").Parse(clashRulesStr)
    if err == nil {
        var buf strings.Builder
        // 注入当前用户的 UUID 及其他节点信息
        err = tmpl.Execute(&buf, map[string]any{
            "UUID":       clientUUID,
            "Host":       host,
            "Proxies":    proxies,
            "ProxyNames": proxyNames,
        })
        if err == nil {
            clashRulesStr = buf.String() // 得到替换变量后的最终 YAML 字符串
        }
    }

    // 调用底层方法，覆盖 config 中的 proxies 和 proxy-groups
    mergeClashRulesYAML(config, clashRulesStr)
}
```

## 3. 实现原理
1. **变量替换**：用户在面板填写 `uuid: {{ .UUID }}`。执行 `tmpl.Execute` 时，自动替换为真实的 `clientUUID` 字符串。
2. **结构覆盖**：替换完毕后的字符串被传入原有的 `mergeClashRulesYAML` 方法。由于该方法基于字典覆盖逻辑 (`base[key] = value`)，只要解析到的 YAML 包含 `proxies` 和 `proxy-groups`，就会直接丢弃系统生成的默认结构，完美实现自定义接管。
