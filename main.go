package main

import (
	"encoding/json"
	"fmt"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Tenant struct {
	gorm.Model
	Name string
}

type Product struct {
	gorm.Model
	TenantID uint
	Title    string
	Price    uint
}

func main() {
	db, err := gorm.Open("postgres", "user=xuser password=xpass dbname=xdb sslmode=disable")
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()

	err = db.DB().Ping()
	if err != nil {
		panic("failed to ping")
	}

	setup(db)

	repo, err := newRepo(db)
	if err != nil {
		panic(err)
	}

	// Read
	product, err := repo.findProduct(3)
	if err != nil {
		panic(err)
	}
	p(product, "Read")

	// Update
	err = repo.updateProduct(product)
	if err != nil {
		panic(err)
	}
	p(product, "Update")

	// Delete
	err = repo.deleteProduct(3)
	if err != nil {
		panic(err)
	}
	_, err = repo.findProduct(3)
	if err != nil {
		if err.Error() == "record not found" {
			fmt.Println("Deleted successfully")
		} else {
			panic(err)
		}
	}
}

// setupでは2回目以降の実行でエラーが出るのでめんどいのでエラー潰してる。
func setup(db *gorm.DB) {
	// Clear table
	db.DropTableIfExists(&Tenant{})
	db.DropTableIfExists(&Product{})

	// Migrate the schema
	db.AutoMigrate(&Tenant{})
	db.AutoMigrate(&Product{})

	// Create records
	{
		tenant := &Tenant{Name: "Apple"}
		db.Create(tenant)
		db.Create(&Product{TenantID: tenant.ID, Title: "Macbook Pro", Price: 250000})
		db.Create(&Product{TenantID: tenant.ID, Title: "iPhoneX", Price: 120000})
	}
	{
		tenant := &Tenant{Name: "Google"}
		db.Create(tenant)
		db.Create(&Product{TenantID: tenant.ID, Title: "Pixel3", Price: 140000})
	}
	{
		tenant := &Tenant{Name: "Amazon"}
		db.Create(tenant)
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon echo", Price: 4500})
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon fireTV", Price: 6000})
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon mini", Price: 2000})
	}

	// Create role
	db.Exec("CREATE ROLE apple")
	db.Exec("CREATE ROLE google")
	db.Exec("CREATE ROLE amazon")

	// Enable RLS
	db.Exec("ALTER TABLE tenants ENABLE ROW LEVEL SECURITY")
	db.Exec("ALTER TABLE products ENABLE ROW LEVEL SECURITY")

	// Create policy
	db.Exec("CREATE POLICY tenants ON id USING(true) WITH CHECK")
}

type repo struct {
	*gorm.DB
}

func newRepo(db *gorm.DB) (*repo, error) {
	return &repo{db}, nil
}

func (db *repo) findProduct(id uint) (*Product, error) {
	var product Product
	err := db.First(&product, 3).Error
	return &product, err
}

func (db *repo) createProduct(product *Product) error {
	return db.Create(product).Error
}

func (db *repo) updateProduct(product *Product) error {
	return db.Model(&product).Update("Price", 99999).Error
}

func (db *repo) deleteProduct(id uint) error {
	product := new(Product)
	product.ID = id
	return db.Delete(product).Error
}

func p(v interface{}, msgs ...string) {
	var msg string
	if len(msgs) > 0 {
		msg = msgs[0]
	}
	fmt.Printf("%s %+v\n", msg, func(v interface{}) string {
		b, _ := json.MarshalIndent(v, "-", "\t")
		return string(b)
	}(v))
}
