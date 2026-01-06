package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "net/http"
    "os"
    "sync"
    
    "github.com/gorilla/sessions"
)

// 用户结构体
type User struct {
    Username string `json:"username"`
    Password string `json:"password"` // 实际应用中应该加密存储
    Email    string `json:"email"`
}

// 存储结构体
type UserStorage struct {
    users map[string]User
    mu    sync.RWMutex
}

// 会话存储
var store = sessions.NewCookieStore([]byte("your-secret-key-here-change-this"))

// 初始化存储
func NewUserStorage() *UserStorage {
    return &UserStorage{
        users: make(map[string]User),
    }
}

// 从文件加载用户数据
func (s *UserStorage) LoadFromFile(filename string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    data, err := os.ReadFile(filename)
    if err != nil {
        // 如果文件不存在，创建默认用户
        if os.IsNotExist(err) {
            defaultUsers := map[string]User{
                "admin": {"admin", "admin123", "admin@example.com"},
                "user":  {"user", "user123", "user@example.com"},
            }
            s.users = defaultUsers
            return s.SaveToFile(filename)
        }
        return err
    }
    
    return json.Unmarshal(data, &s.users)
}

// 保存用户数据到文件
func (s *UserStorage) SaveToFile(filename string) error {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    data, err := json.MarshalIndent(s.users, "", "  ")
    if err != nil {
        return err
    }
    
    return os.WriteFile(filename, data, 0644)
}

// 验证用户
func (s *UserStorage) Authenticate(username, password string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    user, exists := s.users[username]
    return exists && user.Password == password
}

// 添加新用户
func (s *UserStorage) AddUser(username, password, email string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if _, exists := s.users[username]; exists {
        return fmt.Errorf("用户已存在")
    }
    
    s.users[username] = User{
        Username: username,
        Password: password,
        Email:    email,
    }
    
    return nil
}

// 处理器函数
func homeHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session-name")
    
    // 检查是否已登录
    if auth, ok := session.Values["authenticated"].(bool); ok && auth {
        http.Redirect(w, r, "/dashboard", http.StatusFound)
        return
    }
    
    tmpl, err := template.ParseFiles("index.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request, storage *UserStorage) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    err := r.ParseForm()
    if err != nil {
        http.Error(w, "Invalid form data", http.StatusBadRequest)
        return
    }
    
    username := r.FormValue("username")
    password := r.FormValue("password")
    
    if storage.Authenticate(username, password) {
        // 创建会话
        session, _ := store.Get(r, "session-name")
        session.Values["authenticated"] = true
        session.Values["username"] = username
        session.Save(r, w)
        
        http.Redirect(w, r, "/dashboard", http.StatusFound)
    } else {
        // 登录失败，返回错误信息
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprintf(w, `
            <html>
            <body>
                <h2>登录失败</h2>
                <p>用户名或密码错误</p>
                <a href="/">返回登录</a>
            </body>
            </html>
        `)
    }
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session-name")
    session.Values["authenticated"] = false
    session.Save(r, w)
    
    http.Redirect(w, r, "/", http.StatusFound)
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "session-name")
    
    // 检查是否已登录
    if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
        http.Redirect(w, r, "/", http.StatusFound)
        return
    }
    
    username := session.Values["username"].(string)
    
    data := struct {
        Username string
    }{
        Username: username,
    }
    
    tmpl, err := template.ParseFiles("dashboard.html")
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    tmpl.Execute(w, data)
}

func registerHandler(w http.ResponseWriter, r *http.Request, storage *UserStorage) {
    if r.Method == "GET" {
        // 显示注册页面
        tmpl, err := template.ParseFiles("register.html")
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        tmpl.Execute(w, nil)
        return
    }
    
    // 处理注册请求
    if r.Method == "POST" {
        err := r.ParseForm()
        if err != nil {
            http.Error(w, "Invalid form data", http.StatusBadRequest)
            return
        }
        
        username := r.FormValue("username")
        password := r.FormValue("password")
        email := r.FormValue("email")
        
        err = storage.AddUser(username, password, email)
        if err != nil {
            w.Header().Set("Content-Type", "text/html")
            fmt.Fprintf(w, `
                <html>
                <body>
                    <h2>注册失败</h2>
                    <p>%s</p>
                    <a href="/register">返回注册</a>
                </body>
                </html>
            `, err.Error())
            return
        }
        
        // 保存到文件
        if err := storage.SaveToFile("data/users.json"); err != nil {
            log.Printf("保存用户数据失败: %v", err)
        }
        
        w.Header().Set("Content-Type", "text/html")
        fmt.Fprintf(w, `
            <html>
            <body>
                <h2>注册成功</h2>
                <p>用户 %s 注册成功！</p>
                <a href="/">前往登录</a>
            </body>
            </html>
        `, username)
    }
}

func main() {
    // 初始化用户存储
    storage := NewUserStorage()
    if err := storage.LoadFromFile("data/users.json"); err != nil {
        log.Printf("加载用户数据失败: %v", err)
    }
    
    // 创建数据目录
    os.MkdirAll("data", 0755)
    os.MkdirAll("static/css", 0755)
    
    // 设置路由
    http.HandleFunc("/", homeHandler)
    http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        loginHandler(w, r, storage)
    })
    http.HandleFunc("/logout", logoutHandler)
    http.HandleFunc("/dashboard", dashboardHandler)
    http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
        registerHandler(w, r, storage)
    })
    
    // 提供静态文件
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
    
    // 启动服务器
    port := ":8080"
    fmt.Printf("服务器启动在 http://localhost%s\n", port)
    fmt.Println("默认用户:")
    fmt.Println("  用户名: admin, 密码: admin123")
    fmt.Println("  用户名: user, 密码: user123")
    fmt.Println("\n按 Ctrl+C 停止服务器")
    
    log.Fatal(http.ListenAndServe(port, nil))
}