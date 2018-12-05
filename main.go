package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

type Tenant struct {
	gorm.Model
	Key  string
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

	cleanup(db)

	setup(db)

	// ===== Start application-like =====

	var tenants []Tenant
	err = db.Find(&tenants).Error
	if err != nil {
		panic(err)
	}
	p(tenants, "Read all tenants (as super user)")

	// Create connections for each tenants
	conns := make(map[uint]*gorm.DB, len(tenants)) // map[tenantID]*gorm.DB // TODO: consider connection num
	for _, tenant := range tenants {
		udb, err := gorm.Open("postgres", fmt.Sprintf("user=%s password=%s dbname=xdb sslmode=disable", tenant.Key, tenant.Key))
		if err != nil {
			log.Println(err)
			panic(fmt.Sprintf("failed to connect database as %s", tenant.Key))
		}
		conns[tenant.ID] = udb
	}
	defer func() {
		for tid, udb := range conns {
			fmt.Printf("Close db connection: tenant_id: %d\n", tid)
			err := udb.Close()
			if err != nil {
				log.Println(err)
			}
		}
	}()

	// Initialize single repo for multi tenants
	repo, err := newRepo(conns)
	if err != nil {
		panic(err)
	}

	// Test commands to see RLS working correctly
	for _, tenant := range tenants {
		// a user of a tenant is accessing our application right now...

		// Read
		products, err := repo.findAllProducts(tenant.ID)
		if err != nil {
			panic(err)
		}
		p(products, fmt.Sprintf("Read all products of tenant. tid: %d, tname: %s", tenant.ID, tenant.Name))

		//// Update
		//err = repo.updateProduct(product)
		//if err != nil {
		//	panic(err)
		//}
		//p(product, "Update")
		//
		//// Delete
		//err = repo.deleteProduct(1)
		//if err != nil {
		//	panic(err)
		//}
		//_, err = repo.findProduct(1)
		//if err != nil {
		//	if err.Error() == "record not found" {
		//		fmt.Println("Deleted successfully")
		//	} else {
		//		panic(err)
		//	}
		//}

	}
}

func cleanup(db *gorm.DB) {
	// Drop tables (including RLS setting and its policies)
	db.DropTableIfExists(&Tenant{})
	db.DropTableIfExists(&Product{})

	// Drop roles
	db.Exec("DROP ROLE IF EXISTS apple")
	db.Exec("DROP ROLE IF EXISTS google")
	db.Exec("DROP ROLE IF EXISTS amazon")
}

func setup(db *gorm.DB) {
	// Create tables
	db.AutoMigrate(&Tenant{})
	db.AutoMigrate(&Product{})

	// Enable RLS
	db.Exec("ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;")
	db.Exec("ALTER TABLE products ENABLE ROW LEVEL SECURITY;")

	// Create roles(users)
	db.Exec("CREATE ROLE apple WITH LOGIN PASSWORD 'apple'")   // Apple
	db.Exec("CREATE ROLE google WITH LOGIN PASSWORD 'google'") // Google
	db.Exec("CREATE ROLE amazon WITH LOGIN PASSWORD 'amazon'") // Amazon

	// Create policies
	// TODO: role=apple$1 みたいにして USING(id = EXTRACT_ID(current_user)) みたいなことできればポリシー一つで済むようになる
	{
		db.Exec("CREATE POLICY apple_tenants ON tenants TO apple USING(id = 1)")
		db.Exec("CREATE POLICY apple_products ON products TO apple USING(tenant_id = 1)")
	}
	{
		db.Exec("CREATE POLICY google_tenants ON tenants TO google USING(id = 2)")
		db.Exec("CREATE POLICY google_products ON products TO google USING(tenant_id = 2)")
	}
	{
		db.Exec("CREATE POLICY amazon_tenants ON tenants TO amazon USING(id = 3)")
		db.Exec("CREATE POLICY amazon_products ON products TO amazon USING(tenant_id = 3)")
	}

	// Grant privileges
	// TODO: ここテーブル単位でやるのかって感じだからなんか方法考える。
	// 多分親ロールみたいなの作ってそれに各テナントのrole属する形にすれば良さそう。
	{
		db.Exec("GRANT ALL PRIVILEGES ON tenants TO apple")
		db.Exec("GRANT ALL PRIVILEGES ON products TO apple")
	}
	{
		db.Exec("GRANT ALL PRIVILEGES ON tenants TO google")
		db.Exec("GRANT ALL PRIVILEGES ON products TO google")
	}
	{
		db.Exec("GRANT ALL PRIVILEGES ON tenants TO amazon")
		db.Exec("GRANT ALL PRIVILEGES ON products TO amazon")
	}

	// Create records
	{
		tenant := &Tenant{Name: "Apple, Inc", Key: "apple"}
		db.Create(tenant) // id:1
		db.Create(&Product{TenantID: tenant.ID, Title: "Macbook Pro", Price: 250000})
		db.Create(&Product{TenantID: tenant.ID, Title: "iPhoneX", Price: 120000})
	}
	{
		tenant := &Tenant{Name: "Google, Inc", Key: "google"}
		db.Create(tenant) // id:2
		db.Create(&Product{TenantID: tenant.ID, Title: "Pixel3", Price: 140000})
	}
	{
		tenant := &Tenant{Name: "Amazon, Inc", Key: "amazon"}
		db.Create(tenant) // id:3
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon echo", Price: 4500})
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon fireTV", Price: 6000})
		db.Create(&Product{TenantID: tenant.ID, Title: "Amazon mini", Price: 2000})
	}
}

type repo struct {
	conns map[uint]*gorm.DB
}

func newRepo(conns map[uint]*gorm.DB) (*repo, error) {
	return &repo{conns}, nil
}

func (db *repo) findAllProducts(tenantID uint) ([]Product, error) {
	var products []Product
	err := db.conns[tenantID].Find(&products).Error
	return products, err
}

func (db *repo) findProduct(tenantID, id uint) (*Product, error) {
	var product Product
	err := db.conns[tenantID].First(&product, 3).Error
	return &product, err
}

//func (db *repo) createProduct(product *Product) error {
//	return db.Create(product).Error
//}
//
//func (db *repo) updateProduct(product *Product) error {
//	return db.Model(&product).Update("Price", 99999).Error
//}
//
//func (db *repo) deleteProduct(id uint) error {
//	product := new(Product)
//	product.ID = id
//	return db.Delete(product).Error
//}

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
