package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func main() {
	// 支持通过 -p 指定端口，默认 8080
	port := flag.String("p", "8080", "监听端口（默认 8080）")
	flag.Parse()

	// 环境变量可以覆盖端口
	if envPort := os.Getenv("PORT"); envPort != "" {
		*port = envPort
	}

	// 解析 KEYS 环境变量（逗号分割）
	keysEnv := os.Getenv("KEYS")
	var allowedKeys []string
	if keysEnv != "" {
		for _, k := range strings.Split(keysEnv, ",") {
			kk := strings.TrimSpace(k)
			if kk != "" {
				allowedKeys = append(allowedKeys, kk)
			}
		}
	}

	// 将 allowedKeys 存到全局，proxyHandler 会使用它
	_allowedKeys = allowedKeys

	if len(allowedKeys) == 0 {
		_allowedKeys = append(_allowedKeys, "linux-do")
	}

	addr := fmt.Sprintf(":%s", *port)
	http.HandleFunc("/", proxyHandler)
	log.Printf("代理服务启动，监听 %s，所有请求将原样转发到 https://api.deepinfra.com", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("启动失败: %v", err)
	}
}

// 全局允许的 keys 列表
var _allowedKeys []string

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	// 先校验 Authorization: Bearer <key>
	if len(_allowedKeys) > 0 {
		auth := r.Header.Get("Authorization")
		log.Printf("Incoming Authorization header: %q\n", auth)
		log.Printf("Allowed keys: %v\n", _allowedKeys)
		// 回退：如果没有 Authorization 头，尝试从 query 或其他头读取
		if auth == "" {
			// 尝试 X-Api-Key 头
			if ak := r.Header.Get("X-Api-Key"); ak != "" {
				auth = "Bearer " + strings.TrimSpace(ak)
				log.Printf("Found X-Api-Key header, using as token\n")
			}
		}
		if auth == "" {
			// 尝试 query 参数 key 或 api_key
			if qk := r.URL.Query().Get("key"); qk != "" {
				auth = "Bearer " + strings.TrimSpace(qk)
				log.Printf("Found key in query string, using as token\n")
			} else if qk := r.URL.Query().Get("api_key"); qk != "" {
				auth = "Bearer " + strings.TrimSpace(qk)
				log.Printf("Found api_key in query string, using as token\n")
			}
		}
		if auth == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// 支持大小写不同的 Bearer 前缀，按空格分割获取 token
		parts := strings.Fields(auth)
		if len(parts) != 2 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// parts[0] 应为 Bearer（大小写不敏感）
		if !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		token := strings.TrimSpace(parts[1])
		// 去除可能的引号和不可见字符，兼容不同客户端传递格式
		token = strings.Trim(token, "\"'\r\n")
		masked := token
		if len(token) > 6 {
			masked = token[:3] + "..." + token[len(token)-3:]
		}
		log.Printf("Parsed token: %q (masked: %s)\n", token, masked)
		ok := false
		for _, k := range _allowedKeys {
			if k == token {
				ok = true
				break
			}
		}
		if !ok {
			log.Printf("Unauthorized token: %s\n", token)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// 读取并保存请求体，以便转发和打印
	var bodyBytes []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "读取请求体失败", http.StatusInternalServerError)
			return
		}
		bodyBytes = b
		// 重新设置 Body 以便后续使用
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// 构造目标 URL：保持原始路径和查询字符串，目标主机为 api.deepinfra.com
	targetBase := "https://api.deepinfra.com"
	target, err := url.Parse(targetBase)
	if err != nil {
		http.Error(w, "目标地址解析失败", http.StatusInternalServerError)
		return
	}

	// 对特定路径做替换：将 /v1/chat/completions -> /v1/openai/chat/completions，/v1/completions -> /v1/openai/completions
	outPath := r.URL.Path
	if outPath == "/v1/chat/completions" {
		outPath = "/v1/openai/chat/completions"
	}
	if outPath == "/v1/completions" {
		outPath = "/v1/openai/completions"
	}

	// 创建新的 URL 保持查询字符串
	newURL := &url.URL{
		Path:     outPath,
		RawQuery: r.URL.RawQuery,
	}
	destURL := target.ResolveReference(newURL)

	req, err := http.NewRequest(r.Method, destURL.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "创建转发请求失败", http.StatusInternalServerError)
		return
	}

	// 复制原始请求的头，但跳过 Authorization 和 X-Api-Key
	for k, vv := range r.Header {
		lk := strings.ToLower(k)
		if lk == "authorization" || lk == "x-api-key" {
			continue
		}
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	// 然后覆盖为用户提供的特定头（确保 Host 为目标 host）
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://deepinfra.com")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Referer", "https://deepinfra.com/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")
	req.Header.Set("X-Deepinfra-Source", "model-embed")

	// 设置 Host 为目标的 host
	req.Host = target.Host
	req.Header.Set("Host", target.Host)

	// 发起请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "转发请求失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// 将响应体写回给原始客户端
	if _, err := io.Copy(w, resp.Body); err != nil {
		// 日志错误但不再写入客户端
		log.Println("写回响应失败:", err)
	}
}
