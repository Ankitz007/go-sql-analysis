package modules

import (
	"fmt"
	"sync"
	"time"

	"github.com/Ankitz007/go-sql-analysis.git/utils"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func insertFundG(db *gorm.DB, apiData utils.ApiResponse) (uint, error) {
	fund := utils.Fund{
		FundHouse:      apiData.Meta.FundHouse,
		SchemeType:     apiData.Meta.SchemeType,
		SchemeCategory: apiData.Meta.SchemeCategory,
		SchemeCode:     apiData.Meta.SchemeCode,
		SchemeName:     apiData.Meta.SchemeName,
	}

	tx := db.Begin()
	if tx.Error != nil {
		return 0, fmt.Errorf("error starting transaction for Fund: %v", tx.Error)
	}

	if err := tx.Create(&fund).Error; err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("error inserting Fund record: %v", err)
	}

	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("error committing transaction for Fund: %v", err)
	}

	return fund.ID, nil
}

func insertNavRecordsBatchG(db *gorm.DB, navRecords []utils.NavRecord, navBatchSize, concurrency int) error {
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	for i := 0; i < len(navRecords); i += navBatchSize {
		end := i + navBatchSize
		if end > len(navRecords) {
			end = len(navRecords)
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(start, end int) {
			defer wg.Done()
			defer func() { <-sem }()

			tx := db.Begin()
			if tx.Error != nil {
				fmt.Printf("Error starting transaction for NavRecord batch %d to %d: %v\n", start, end-1, tx.Error)
				return
			}

			if err := bulkInsertNavRecords(tx, navRecords[start:end]); err != nil {
				fmt.Printf("Error inserting NavRecord batch %d to %d: %v\n", start, end-1, err)
				tx.Rollback()
				return
			}

			if err := tx.Commit().Error; err != nil {
				fmt.Printf("Error committing transaction for NavRecord batch %d to %d: %v\n", start, end-1, err)
				return
			}
		}(i, end)
	}

	wg.Wait()
	return nil
}

func parseNavRecords(apiData utils.ApiResponse, fundID uint) ([]utils.NavRecord, error) {

	var navRecords []utils.NavRecord
	for _, record := range apiData.Data {
		date, err := time.Parse("02-01-2006", record.Date)
		if err != nil {
			return nil, fmt.Errorf("error parsing date %s: %v", record.Date, err)
		}

		navRecords = append(navRecords, utils.NavRecord{
			Date:   date,
			Nav:    utils.AtoF(record.Nav),
			FundID: fundID,
		})
	}
	return navRecords, nil
}

func runG(db *gorm.DB, navBatchSize, concurrency int) {

	apiData, err := utils.FetchDataFromAPI()
	if err != nil {
		fmt.Printf("Error fetching data from API: %v\n", err)
		return
	}

	fundID, err := insertFundG(db, apiData)
	if err != nil {
		fmt.Printf("Error inserting Fund record: %v\n", err)
		return
	}
	fmt.Println("Fund inserted successfully")

	navRecords, err := parseNavRecords(apiData, fundID)
	if err != nil {
		fmt.Printf("Error parsing NAV records: %v\n", err)
		return
	}

	err = insertNavRecordsBatchG(db, navRecords, navBatchSize, concurrency)
	if err != nil {
		fmt.Printf("Error inserting NAV records: %v\n", err)
		return
	}

	fmt.Println("Data inserted successfully.")
}

func bulkInsertNavRecords(tx *gorm.DB, navRecords []utils.NavRecord) error {
	return tx.Create(&navRecords).Error
}

func RunGorm() {
	start := time.Now()
	dsn := "myuser:mypassword@tcp(<YOUR_IP>)/funds?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		fmt.Printf("Error opening connection: %v\n", err)
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		fmt.Printf("Error getting database instance: %v\n", err)
		return
	}

	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Minute)

	connTime := time.Since(start)

	const batchSize = 1000
	const concurrency = 8

	runG(db, batchSize, concurrency)

	fmt.Printf("Connection: %s Insertion: %s Total: %s\n", connTime, time.Since(start)-connTime, time.Since(start))
}
