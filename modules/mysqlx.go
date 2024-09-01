package modules

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ankitz007/go-sql-analysis.git/utils"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func connect(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error opening connection: %v", err)
	}

	db.SetMaxIdleConns(20)
	db.SetMaxOpenConns(100)
	db.SetConnMaxLifetime(time.Minute)

	return db, nil
}

func insertFundS(db *sqlx.DB, fund utils.Fund) (int64, error) {
	tx, err := db.Beginx()
	if err != nil {
		return 0, fmt.Errorf("error starting transaction for fund: %v", err)
	}

	insertFundQuery := "INSERT INTO funds (fund_house, scheme_type, scheme_category, scheme_code, scheme_name) VALUES (?, ?, ?, ?, ?)"
	result, err := tx.Exec(insertFundQuery, fund.FundHouse, fund.SchemeType, fund.SchemeCategory, fund.SchemeCode, fund.SchemeName)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("error inserting fund: %v", err)
	}

	fundID, err := result.LastInsertId()
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("error retrieving last insert ID: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("error committing transaction for fund: %v", err)
	}
	fmt.Println("Fund inserted successfully")

	return fundID, nil
}

func insertNavRecordsBatch(db *sqlx.DB, navRecords []utils.NavRecord, wg *sync.WaitGroup, limitCh chan struct{}) {
	defer wg.Done()

	limitCh <- struct{}{}
	defer func() { <-limitCh }()

	tx, err := db.Beginx()
	if err != nil {
		fmt.Printf("Error starting transaction for nav_records batch: %v\n", err)
		return
	}

	insertNavRecordsQuery := "INSERT INTO nav_records (fund_id, date, nav) VALUES "
	navRecordValues := make([]interface{}, 0, len(navRecords)*3)

	for i, record := range navRecords {
		if i > 0 {
			insertNavRecordsQuery += ", "
		}
		insertNavRecordsQuery += "(?, ?, ?)"
		navRecordValues = append(navRecordValues, record.FundID, record.Date, record.Nav)
	}

	stmtNavRecords, err := tx.Preparex(insertNavRecordsQuery)
	if err != nil {
		fmt.Printf("Error preparing statement for nav_records batch: %v\n", err)
		tx.Rollback()
		return
	}
	defer stmtNavRecords.Close()

	_, err = stmtNavRecords.Exec(navRecordValues...)
	if err != nil {
		fmt.Printf("Error inserting nav_records batch: %v\n", err)
		tx.Rollback()
		return
	}

	err = tx.Commit()
	if err != nil {
		fmt.Printf("Error committing transaction for nav_records batch: %v\n", err)
		return
	}
}

func runS(db *sqlx.DB, batchSize, concurrencyLimit int) {

	apiResponse, err := utils.FetchDataFromAPI()
	if err != nil {
		fmt.Printf("Error fetching data from API: %v\n", err)
		return
	}

	fund := utils.Fund{
		FundHouse:      apiResponse.Meta.FundHouse,
		SchemeType:     apiResponse.Meta.SchemeType,
		SchemeCategory: apiResponse.Meta.SchemeCategory,
		SchemeCode:     apiResponse.Meta.SchemeCode,
		SchemeName:     apiResponse.Meta.SchemeName,
	}

	fundID, err := insertFundS(db, fund)
	if err != nil {
		fmt.Printf("Error inserting fund: %v\n", err)
		return
	}

	var navRecords []utils.NavRecord

	for _, record := range apiResponse.Data {
		date, err := time.Parse("02-01-2006", record.Date)
		if err != nil {
			fmt.Printf("Error parsing date %s: %v\n", record.Date, err)
			continue
		}
		nav := utils.NavRecord{
			FundID: uint(fundID),
			Date:   date,
			Nav:    utils.AtoF(record.Nav),
		}
		navRecords = append(navRecords, nav)
	}

	limitCh := make(chan struct{}, concurrencyLimit)
	var wg sync.WaitGroup

	for i := 0; i < len(navRecords); i += batchSize {
		end := i + batchSize
		if end > len(navRecords) {
			end = len(navRecords)
		}
		batchNavRecords := navRecords[i:end]

		wg.Add(1)
		go insertNavRecordsBatch(db, batchNavRecords, &wg, limitCh)
	}

	wg.Wait()

	fmt.Println("Data inserted successfully.")
}

func RunSqlx() {
	start := time.Now()
	dsn := "myuser:mypassword@tcp(<YOUR_IP>)/funds?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := connect(dsn)
	if err != nil {
		fmt.Printf("error creating table schema: %v", err)
		return
	}

	connTime := time.Since(start)

	const batchSize = 1000
	const concurrencyLimit = 8

	runS(db, batchSize, concurrencyLimit)

	fmt.Printf("Connection: %s Insertion: %s Total: %s\n", connTime, time.Since(start)-connTime, time.Since(start))
}
