package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "sync"
)

// ... 其他代码保持不变 ...

func main() {
    // 初始化存储
    storage := NewUserStorage()
    if err := storage.LoadFromFile("data/users.json"); err != nil {
        log.Printf("加载用户数据失败: %v", err)
    }
    
    // 创建目录
    os.MkdirAll("data", 0755)
    os.MkdirAll("static/css", 0755)
    
    // 设置路由 - 修复这里！
    mux := http.NewServeMux()
    
    // 静态文件
    mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
    
    // 页面路由
    mux.HandleFunc("/", homeHandler)
    mux.HandleFunc("/dashboard", dashboardHandler)
    mux.HandleFunc("/logout", logoutHandler)
    mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
        registerHandler(w, r, storage)
    })
    
    // 登录路由 - 重要修复！
    mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        log.Printf("收到 %s 请求 /login", r.Method)
        
        if r.Method == "GET" {
            // 如果是GET请求，重定向到首页
            http.Redirect(w, r, "/", http.StatusFound)
            return
        }
        
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        // 解析表单数据
        if err := r.ParseForm(); err != nil {
            http.Error(w, "表单解析错误", http.StatusBadRequest)
            return
        }
        
        username := r.FormValue("username")
        password := r.FormValue("password")
        
        log.Printf("尝试登录: 用户=%s, 密码长度=%d", username, len(password))
        
        if storage.Authenticate(username, password) {
            // 创建会话
            session, _ := store.Get(r, "session-name")
            session.Values["authenticated"] = true
            session.Values["username"] = username
            if err := session.Save(r, w); err != nil {
                log.Printf("保存会话失败: %v", err)
            }
            
            log.Printf("用户 %s 登录成功", username)
            http.Redirect(w, r, "/dashboard", http.StatusFound)
        } else {
            log.Printf("用户 %s 登录失败", username)
            // 返回登录页面并显示错误
            w.Header().Set("Content-Type", "text/html; charset=utf-8")
            w.WriteHeader(http.StatusUnauthorized)
            fmt.Fprintf(w, `
                <!DOCTYPE html>
                <html>
                <head>
                    <title>登录失败</title>
                    <style>
                        body { font-family: Arial, sans-serif; padding: 20px; }
                        .error { color: red; margin: 20px 0; }
                        a { color: #4CAF50; }
                    </style>
                </head>
                <body>
                    <h2>登录失败</h2>
                    <div class="error">用户名或密码错误</div>
                    <p><a href="/">返回登录页面</a></p>
                </body>
                </html>
            `)
        }
    })
    
    // 启动服务器
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    
    log.Printf("服务器启动在端口 %s", port)
    log.Fatal(http.ListenAndServe(":"+port, mux))
}

// 简化版的会话处理
var store = sessions.NewCookieStore([]byte("your-secret-key"))

// 确保所有处理器都存在
func homeHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    
    tmpl, err := template.ParseFiles("index.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, nil)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session-name")
    if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
        http.Redirect(w, r, "/", http.StatusFound)
        return
    }
    
    username := session.Values["username"].(string)
    data := struct{ Username string }{Username: username}
    
    tmpl, err := template.ParseFiles("dashboard.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, data)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session-name")
    session.Values["authenticated"] = false
    session.Save(r, w)
    http.Redirect(w, r, "/", http.StatusFound)
}
