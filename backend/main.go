package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// --- STRUCTS / MODELS ---

type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Slug      string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Products  []Product `gorm:"foreignKey:CategoryID" json:"products,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Product struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Slug        string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Type        string    `gorm:"size:100;index" json:"type"`
	Price       float64   `gorm:"not null" json:"price"`
	SalePrice   float64   `gorm:"default:0" json:"sale_price"`
	Image       string    `gorm:"size:500" json:"image"`
	Description string    `gorm:"type:text" json:"description"`
	Dimensions  string    `gorm:"size:255" json:"dimensions"`
	Material    string    `gorm:"size:255" json:"material"`
	Stock       int       `gorm:"default:0" json:"stock"`
	CategoryID  uint      `json:"category_id"`
	Category    Category  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Page struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Title       string    `gorm:"size:255;not null" json:"title"`
	Slug        string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	HtmlContent string    `gorm:"type:longtext" json:"html_content"`
	MetaData    string    `gorm:"type:text" json:"meta_data"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:100;uniqueIndex;not null" json:"username"`
	Name      string    `gorm:"size:255" json:"name"`
	Password  string    `gorm:"size:255;not null" json:"-"`
	Email     string    `gorm:"size:255;uniqueIndex" json:"email"`
	GoogleID  *string   `gorm:"size:255;uniqueIndex" json:"-"`
	Role      string    `gorm:"size:50;default:'admin'" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var db *gorm.DB

func main() {
	dsn := "root:@tcp(127.0.0.1:3307)/noithat_db?charset=utf8mb4&parseTime=True&loc=Local"
	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		panic("❌ Kết nối Database thất bại: " + err.Error())
	}
	fmt.Println(" Kết nối MySQL (Port 3307) thành công!")

	db.AutoMigrate(&Category{}, &Product{}, &Page{}, &User{})

	// Tự động khởi tạo dữ liệu mẫu khi backend khởi chạy
	seedData()

	r := gin.Default()

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
			return
		}
		c.Next()
	})

	// --- ROUTES API ---
	r.POST("/api/auth/register", registerUser)
	r.POST("/api/auth/login", loginUser)
	r.POST("/api/auth/google", googleLogin)
	r.GET("/api/auth/me", currentUser)

	// 1. API Test
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Backend Golang đang chạy thành công!"})
	})

	// 2. API Lấy danh sách sản phẩm (có lọc theo type hoặc category)
	r.GET("/api/products", func(c *gin.Context) {
		var products []Product
		query := db.Preload("Category")
		productType := c.Query("type")
		if productType != "" {
			query = query.Where("type = ? OR slug LIKE ?", productType, "%"+productType+"%")
		}
		category := c.Query("category")
		if category != "" {
			query = query.Joins("Category").Where("categories.slug = ?", category)
		}
		query.Find(&products)
		c.JSON(http.StatusOK, products)
	})

	// 2.1 API Thêm mới sản phẩm (Dùng để nhập thông tin sản phẩm mới)
	r.POST("/api/products", func(c *gin.Context) {
		var input Product
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()})
			return
		}
		if input.Name == "" || input.Slug == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Tên và slug sản phẩm không được để trống."})
			return
		}
		if err := db.Create(&input).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi khi lưu sản phẩm: " + err.Error()})
			return
		}
		db.Preload("Category").First(&input, input.ID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "Thêm sản phẩm thành công!",
			"product": input,
		})
	})

	// 3. API Lấy chi tiết 1 sản phẩm theo Slug
	r.GET("/api/products/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		var product Product
		if err := db.Preload("Category").Where("slug = ?", slug).First(&product).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Không tìm thấy sản phẩm"})
			return
		}
		c.JSON(http.StatusOK, product)
	})

	// 4. API Tạo dữ liệu mẫu nội thất (Seed Data)
	r.POST("/api/seed", func(c *gin.Context) {
		seedData()
		c.JSON(http.StatusOK, gin.H{"message": "Đã khởi tạo dữ liệu mẫu thành công!"})
	})

	// 5. API Lấy Custom Page theo Slug
	r.GET("/api/pages/:slug", func(c *gin.Context) {
		slug := c.Param("slug")
		var page Page
		if err := db.Where("slug = ? AND is_active = ?", slug, true).First(&page).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Trang không tồn tại"})
			return
		}
		c.JSON(http.StatusOK, page)
	})

	// 6. API Seed Page mẫu (Giới thiệu, Khuyến mãi)
	r.POST("/api/seed-pages", func(c *gin.Context) {
		seedData()
		c.JSON(http.StatusOK, gin.H{"message": "Đã khởi tạo trang HTML mẫu thành công!"})
	})
	r.Run(":8080")
}

type authRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

const defaultGoogleClientID = "169312903386-mrd6elqubhm99jmouhrsjf9ho0k7d3dp.apps.googleusercontent.com"

type googleAuthRequest struct {
	Credential string `json:"credential"`
}

func googleLogin(c *gin.Context) {
	var input googleAuthRequest
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Credential) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Thiếu thông tin xác thực Google."})
		return
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		clientID = defaultGoogleClientID
	}
	payload, err := idtoken.Validate(c.Request.Context(), input.Credential, clientID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không xác minh được tài khoản Google. Hãy thử lại."})
		return
	}
	email, _ := payload.Claims["email"].(string)
	name, _ := payload.Claims["name"].(string)
	emailVerified, _ := payload.Claims["email_verified"].(bool)
	if payload.Subject == "" || strings.TrimSpace(email) == "" || !emailVerified {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Google chưa xác nhận địa chỉ email của tài khoản này."})
		return
	}

	email = strings.ToLower(strings.TrimSpace(email))
	googleID := payload.Subject
	var user User
	err = db.Where("google_id = ?", googleID).First(&user).Error
	if err == nil {
		user.Email = email
		if name != "" {
			user.Name = name
		}
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể cập nhật tài khoản lúc này."})
			return
		}
		respondWithAuth(c, user)
		return
	}
	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tra cứu tài khoản lúc này."})
		return
	}

	// A verified Google email can link to an existing local account with the same email.
	if err := db.Where("email = ?", email).First(&user).Error; err == nil {
		user.GoogleID = &googleID
		if name != "" {
			user.Name = name
		}
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Tài khoản email này không thể liên kết với Google."})
			return
		}
		respondWithAuth(c, user)
		return
	} else if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tra cứu tài khoản lúc này."})
		return
	}

	username := "google_" + googleID
	if len(username) > 100 {
		username = username[:100]
	}
	if name == "" {
		name = email
	}
	user = User{Username: username, Name: name, Email: email, GoogleID: &googleID, Role: "customer"}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Không thể tạo tài khoản Google lúc này."})
		return
	}
	respondWithAuth(c, user)
}

func registerUser(c *gin.Context) {
	var input authRequest
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Username) == "" || strings.TrimSpace(input.Email) == "" || len(input.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập tên, email và mật khẩu từ 8 ký tự."})
		return
	}
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var existing User
	if err := db.Where("email = ? OR username = ?", input.Email, input.Username).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email hoặc tên đăng nhập đã được sử dụng."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Không thể tạo tài khoản lúc này."})
		return
	}
	user := User{Username: input.Username, Name: input.Username, Email: input.Email, Password: string(hash), Role: "customer"}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email hoặc tên đăng nhập đã được sử dụng."})
		return
	}
	respondWithAuth(c, user)
}

func loginUser(c *gin.Context) {
	var input authRequest
	if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Email) == "" || input.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng nhập email và mật khẩu."})
		return
	}
	var user User
	identifier := strings.ToLower(strings.TrimSpace(input.Email))
	if identifier == "" {
		identifier = strings.TrimSpace(input.Username)
	}
	if err := db.Where("email = ? OR username = ?", identifier, identifier).First(&user).Error; err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Email hoặc mật khẩu không chính xác."})
		return
	}
	respondWithAuth(c, user)
}

func respondWithAuth(c *gin.Context, user User) {
	expires := time.Now().Add(24 * time.Hour).Unix()
	payload, _ := json.Marshal(map[string]interface{}{"id": user.ID, "username": user.Username, "exp": expires})
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, authSecret())
	mac.Write([]byte(encoded))
	token := encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	c.JSON(http.StatusOK, gin.H{"token": token, "expires_at": expires, "user": gin.H{"id": user.ID, "username": user.Username, "name": user.Name, "email": user.Email}})
}

func currentUser(c *gin.Context) {
	parts := strings.SplitN(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "), ".", 2)
	if len(parts) != 2 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ."})
		return
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	mac := hmac.New(sha256.New, authSecret())
	mac.Write([]byte(parts[0]))
	if err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập không hợp lệ."})
		return
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	var claims struct {
		ID  uint  `json:"id"`
		Exp int64 `json:"exp"`
	}
	if err != nil || json.Unmarshal(data, &claims) != nil || claims.Exp < time.Now().Unix() {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Phiên đăng nhập đã hết hạn."})
		return
	}
	var user User
	if db.First(&user, claims.ID).Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Không tìm thấy tài khoản."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "name": user.Name, "email": user.Email}})
}

func authSecret() []byte {
	if secret := os.Getenv("AUTH_SECRET"); secret != "" {
		return []byte(secret)
	}
	return []byte("noithat-dev-only-change-this-secret")
}

func seedData() {
	// 1. Seed Categories & Products
	var catSofa Category
	if err := db.Where("slug = ?", "sofa-ghe").First(&catSofa).Error; err != nil {
		catSofa = Category{Name: "Sofa & Ghế", Slug: "sofa-ghe"}
		db.Create(&catSofa)
	}

	var catBan Category
	if err := db.Where("slug = ?", "ban-an").First(&catBan).Error; err != nil {
		catBan = Category{Name: "Bàn Ăn", Slug: "ban-an"}
		db.Create(&catBan)
	}

	products := []Product{
		{
			Name:        "Sofa Băng Bọc Da Cao Cấp KHÔNG GIAN MỚI",
			Slug:        "sofa-bang-boc-da-khonggianmoi",
			Type:        "sofa-bang",
			Price:       15500000,
			SalePrice:   12900000,
			Image:       "https://images.unsplash.com/photo-1555041469-a586c61ea9bc?w=800",
			Description: "Sofa bọc da Ý cao cấp, khung gỗ sồi tự nhiên chống mối mọt.",
			Dimensions:  "220cm x 90cm x 85cm",
			Material:    "Da thật & Gỗ Sồi",
			Stock:       10,
			CategoryID:  catSofa.ID,
		},
		{
			Name:        "Bàn Ăn Gỗ Sồi Nguyên Khối 6 Ghế",
			Slug:        "ban-an-go-soi-nguyen-khoi",
			Type:        "ban-an",
			Price:       8900000,
			SalePrice:   7500000,
			Image:       "https://images.unsplash.com/photo-1615066390971-03e4e1c36ddf?w=800",
			Description: "Bàn ăn 6 ghế thiết kế tối giản, phong cách Bắc Âu chuẩn Scandinavian.",
			Dimensions:  "180cm x 80cm x 75cm",
			Material:    "Gỗ Sồi Nga Tự Nhiên",
			Stock:       8,
			CategoryID:  catBan.ID,
		},
		{
			Name:        "Bàn Ăn Mặt Đá Ceramic Chống Xước",
			Slug:        "ban-an-mat-da-ceramic",
			Type:        "ban-an",
			Price:       12500000,
			SalePrice:   10800000,
			Image:       "https://images.unsplash.com/photo-1530018607912-eff2daa1bac4?w=800",
			Description: "Mặt đá ceramic cao cấp chịu nhiệt, chống ố vàng và chống trầy xước tuyệt đối.",
			Dimensions:  "160cm x 85cm x 75cm",
			Material:    "Đá Ceramic & Khung Thép Sơn Tĩnh Điện",
			Stock:       6,
			CategoryID:  catBan.ID,
		},
		{
			Name:        "Bàn Ăn Tròn Mở Rộng Thông Minh",
			Slug:        "ban-an-tron-thong-minh",
			Type:        "ban-an",
			Price:       14200000,
			SalePrice:   11900000,
			Image:       "https://images.unsplash.com/photo-1577140917170-285929fb55b7?w=800",
			Description: "Bàn ăn tròn hiện đại có thể kéo dài linh hoạt từ 4 đến 8 người ngồi.",
			Dimensions:  "Đường kính 120cm - 160cm",
			Material:    "Gỗ Óc Chó (Walnut)",
			Stock:       4,
			CategoryID:  catBan.ID,
		},
		{
			Name:        "Bàn Ăn Mango 4 Ghế Nhỏ Gọn",
			Slug:        "ban-an-mango-4-ghe",
			Type:        "ban-an",
			Price:       5600000,
			SalePrice:   4500000,
			Image:       "https://images.unsplash.com/photo-1617806118233-18e1de247200?w=800",
			Description: "Thiết kế nhỏ gọn, đường cong bo tròn an toàn, phù hợp căn hộ chung cư vừa và nhỏ.",
			Dimensions:  "120cm x 75cm x 75cm",
			Material:    "Gỗ Cao Su Tự Nhiên",
			Stock:       12,
			CategoryID:  catBan.ID,
		},
	}

	for _, p := range products {
		var existing Product
		if err := db.Where("slug = ?", p.Slug).First(&existing).Error; err != nil {
			db.Create(&p)
		} else {
			existing.Type = p.Type
			existing.Price = p.Price
			existing.SalePrice = p.SalePrice
			existing.Image = p.Image
			existing.Description = p.Description
			existing.Material = p.Material
			existing.Dimensions = p.Dimensions
			existing.Stock = p.Stock
			existing.CategoryID = p.CategoryID
			db.Save(&existing)
		}
	}

	// 2. Seed Page Giới thiệu
	aboutPage := Page{
		Title:    "Về Thương Hiệu KHÔNG GIAN MỚI",
		Slug:     "gioi-thieu",
		MetaData: "{}",
		HtmlContent: `<div class="prose max-w-none">
			<h2 class="text-2xl font-bold text-amber-900 mb-4">Phong Cách Thiết Kế KHÔNG GIAN MỚI</h2>
			<p class="mb-4">KHÔNG GIAN MỚI được khởi nguồn từ niềm đam mê tối giản, kết hợp giữa chất liệu gỗ tự nhiên tinh tế và đường nét hiện đại.</p>
			<blockquote class="border-l-4 border-amber-800 pl-4 italic my-4 text-gray-700">"Nội thất không chỉ để ngắm, mà là để sống cùng."</blockquote>
			<p>Chúng tôi tự hào đem đến cho không gian sống của bạn sự ấm cúng và sang trọng vượt thời gian.</p>
		</div>`,
		IsActive: true,
	}

	var existingPage Page
	if err := db.Where("slug = ?", aboutPage.Slug).First(&existingPage).Error; err != nil {
		db.Create(&aboutPage)
	} else {
		existingPage.Title = aboutPage.Title
		existingPage.HtmlContent = aboutPage.HtmlContent
		existingPage.IsActive = true
		db.Save(&existingPage)
	}
}
