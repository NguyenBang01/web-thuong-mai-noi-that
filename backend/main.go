package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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
	Password  string    `gorm:"size:255;not null" json:"-"`
	Email     string    `gorm:"size:255;uniqueIndex" json:"email"`
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

	// 1. API Test
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Backend Golang đang chạy thành công!"})
	})

	// 2. API Lấy danh sách sản phẩm
	r.GET("/api/products", func(c *gin.Context) {
		var products []Product
		db.Preload("Category").Find(&products)
		c.JSON(http.StatusOK, products)
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
			Name:        "Sofa Băng Bọc Da Cao Cấp NAMQUAN",
			Slug:        "sofa-bang-boc-da-namquan",
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
			Name:        "Bàn Ăn Gỗ Sồi Nguyên Khối",
			Slug:        "ban-an-go-soi-nguyen-khoi",
			Price:       8900000,
			SalePrice:   7500000,
			Image:       "https://images.unsplash.com/photo-1615066390971-03e4e1c36ddf?w=800",
			Description: "Bàn ăn 6 ghế thiết kế tối giản, phong cách Bắc Âu.",
			Dimensions:  "180cm x 80cm x 75cm",
			Material:    "Gỗ Sồi Nga",
			Stock:       5,
			CategoryID:  catBan.ID,
		},
	}

	for _, p := range products {
		var existing Product
		if err := db.Where("slug = ?", p.Slug).First(&existing).Error; err != nil {
			db.Create(&p)
		}
	}

	// 2. Seed Page Giới thiệu
	aboutPage := Page{
		Title:    "Về Thương Hiệu NAMQUAN",
		Slug:     "gioi-thieu",
		MetaData: "{}",
		HtmlContent: `<div class="prose max-w-none">
			<h2 class="text-2xl font-bold text-amber-900 mb-4">Phong Cách Thiết Kế NAMQUAN</h2>
			<p class="mb-4">NAMQUAN được khởi nguồn từ niềm đam mê tối giản, kết hợp giữa chất liệu gỗ tự nhiên tinh tế và đường nét hiện đại.</p>
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
