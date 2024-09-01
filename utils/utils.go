package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"gorm.io/gorm"
)

func AtoF(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		fmt.Printf("Error converting string to float: %v\n", err)
		return 0
	}
	return f
}

func TruncateTables(db interface{}) error {
	switch db := db.(type) {
	case *sqlx.DB:
		_, err := db.Exec("SET FOREIGN_KEY_CHECKS = 0;TRUNCATE TABLE funds;TRUNCATE TABLE nav_records;SET FOREIGN_KEY_CHECKS = 1;")
		if err != nil {
			return fmt.Errorf("error truncating tables: %v", err)
		}
	case *gorm.DB:
		err := db.Exec("SET FOREIGN_KEY_CHECKS = 0;TRUNCATE TABLE funds;TRUNCATE TABLE nav_records;SET FOREIGN_KEY_CHECKS = 1;").Error
		if err != nil {
			return fmt.Errorf("error truncating tables: %v", err)
		}
	default:
		return fmt.Errorf("unsupported database type")
	}
	return nil
}

type Fund struct {
	ID             uint   `db:"id"`
	FundHouse      string `db:"fund_house"`
	SchemeType     string `db:"scheme_type"`
	SchemeCategory string `db:"scheme_category"`
	SchemeCode     int    `db:"scheme_code"`
	SchemeName     string `db:"scheme_name"`
}

type NavRecord struct {
	ID     uint      `db:"id"`
	FundID uint      `db:"fund_id"`
	Date   time.Time `db:"date"`
	Nav    float64   `db:"nav"`
}

type ApiResponse struct {
	Meta struct {
		FundHouse      string `json:"fund_house"`
		SchemeType     string `json:"scheme_type"`
		SchemeCategory string `json:"scheme_category"`
		SchemeCode     int    `json:"scheme_code"`
		SchemeName     string `json:"scheme_name"`
	} `json:"meta"`
	Period string `json:"period"`
	Data   []struct {
		Date string `json:"date"`
		Nav  string `json:"nav"`
	} `json:"data"`
}

func FetchDataFromAPI() (ApiResponse, error) {
	resp, err := http.Get(apiUrl)
	if err != nil {
		return ApiResponse{}, fmt.Errorf("error fetching data from API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ApiResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ApiResponse{}, fmt.Errorf("error reading response body: %w", err)
	}

	var data ApiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return ApiResponse{}, fmt.Errorf("error parsing JSON: %w", err)
	}

	return data, nil
}

func FetchFundData(db interface{}, fundID uint) error {
	var fund Fund
	var navRecords []NavRecord

	switch db := db.(type) {
	case *sqlx.DB:

		err := db.Get(&fund, "SELECT * FROM funds WHERE id = ?", fundID)
		if err != nil {
			return fmt.Errorf("error fetching fund: %v", err)
		}

		err = db.Select(&navRecords, "SELECT * FROM nav_records WHERE fund_id = ?", fundID)
		if err != nil {
			return fmt.Errorf("error fetching nav records: %v", err)
		}

	case *gorm.DB:

		err := db.Where("id = ?", fundID).First(&fund).Error
		if err != nil {
			return fmt.Errorf("error fetching fund: %v", err)
		}

		err = db.Where("fund_id = ?", fundID).Find(&navRecords).Error
		if err != nil {
			return fmt.Errorf("error fetching nav records: %v", err)
		}

	default:
		return fmt.Errorf("unsupported database type")
	}

	apiResponse := ApiResponse{
		Period: "monthly",
	}
	apiResponse.Meta.FundHouse = fund.FundHouse
	apiResponse.Meta.SchemeType = fund.SchemeType
	apiResponse.Meta.SchemeCategory = fund.SchemeCategory
	apiResponse.Meta.SchemeCode = int(fund.SchemeCode)
	apiResponse.Meta.SchemeName = fund.SchemeName

	for _, record := range navRecords {
		apiResponse.Data = append(apiResponse.Data, struct {
			Date string `json:"date"`
			Nav  string `json:"nav"`
		}{
			Date: record.Date.Format("02-01-2006"),
			Nav:  fmt.Sprintf("%.2f", record.Nav),
		})
	}

	data, err := json.MarshalIndent(apiResponse, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshalling JSON: %v", err)
	}

	log.Println(string(data))
	return nil
}
