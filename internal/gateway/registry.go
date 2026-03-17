package gateway

import "net/url"

// ServiceEntry 注册表中的一条记录：路由前缀 → 下游服务地址。
// ServiceEntry is one registry record: route prefix → downstream service URL.
type ServiceEntry struct {
	Prefix    string   // 路由前缀，如 "/api/v1/user/" / Route prefix, e.g. "/api/v1/user/"
	TargetURL *url.URL // 下游服务地址，如 "http://user:3001" / Downstream URL, e.g. "http://user:3001"
}

// Registry 服务注册表，维护路由前缀到下游服务的映射。
// Registry maintains a mapping from route prefixes to downstream services.
//
// 为什么用 slice 而不是 map？因为需要按注册顺序匹配（最先匹配的优先），
// 而且前缀数量很少（<10），遍历比 map 更简单可控。
// Why a slice instead of a map? We need ordered matching (first match wins),
// and with <10 prefixes, iteration is simpler and more predictable than a map.
type Registry struct {
	entries []ServiceEntry
}

// NewRegistry 创建空的服务注册表。
// NewRegistry creates an empty service registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register 注册一个路由前缀到下游服务地址的映射。
// Register adds a route prefix → downstream service URL mapping.
//
// rawURL 是下游服务的基础地址，如 "http://user:3001"。
// rawURL is the downstream service's base URL, e.g. "http://user:3001".
func (reg *Registry) Register(prefix, rawURL string) error {
	// url.Parse 把字符串解析为结构化的 URL 对象。
	// url.Parse converts a string into a structured URL object.
	//
	// 为什么在注册时解析而不是每次请求时解析？注册只做一次（启动时），
	// 解析结果缓存在 entry 里，避免每次请求重复解析。
	// Why parse at registration instead of per request? Registration happens once (at startup),
	// and the parsed result is cached in the entry, avoiding repeated parsing per request.
	target, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	reg.entries = append(reg.entries, ServiceEntry{
		Prefix:    prefix,
		TargetURL: target,
	})
	return nil
}

// Lookup 根据请求路径找到匹配的下游服务。
// Lookup finds the downstream service matching the given request path.
//
// 遍历注册表，找到第一个前缀匹配的 entry。
// Iterates through the registry and returns the first prefix match.
func (reg *Registry) Lookup(path string) *ServiceEntry {
	for i := range reg.entries {
		// strings.HasPrefix 的效果，但直接用 len 比较更轻量。
		// Same effect as strings.HasPrefix, but comparing with len is lighter.
		if len(path) >= len(reg.entries[i].Prefix) && path[:len(reg.entries[i].Prefix)] == reg.entries[i].Prefix {
			return &reg.entries[i]
		}
	}
	return nil
}
