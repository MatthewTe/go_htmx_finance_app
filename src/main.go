package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RequestData struct {
	UniqueId string `json:"uniqueId"`
}

func mainHandler(w http.ResponseWriter, r *http.Request) {

	dbPath := "./finance_database.sqlite"

	/*

		db, err := RebuildDatabase(dbPath)
		if err != nil {
			log.Fatal(err)
		}

			_, err = LoadTestData("../data/test_transactions.csv", db)
			if err != nil {
				log.Fatal(err)
			}

			db, err := sql.Open("sqlite3", dbPath)
			if err != nil {
				log.Fatal(err)
			}
	*/
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	transactions, err := ReadAllTransactions(db)
	if err != nil {
		log.Fatal("Unable to extract all transactions from the database")
	}

	if len(transactions) == 0 {
		tmpl, err := template.ParseFiles("../templates/index.html")
		if err != nil {
			log.Fatal("Unable to load the index.html template: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmpl.Execute(w, nil)
		if err != nil {

			if strings.Contains(err.Error(), "broken pipe") {
				log.Println("Client closed the connection prematurely")
				return
			}

			log.Fatal("Unable to render the index.html template: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Resampling Transactions for daily timeseries:
	resampleTransactionTimeseries, err := LoadBudgetFromCSV(transactions)
	if err != nil {
		log.Fatal("Unable to resample the transaction timeseries")
	}

	// Destructuring the dailyResample component to get the timeseries arrays to pass to template from map:
	var resampledDatetime = []time.Time{}
	var resampledIncome = []float64{}
	var resampledExpense = []float64{}
	var resampledBalance = []float64{}

	// Generating the statistics from the budget:
	var TotalIncome float64 = 0.0
	var TotalExpenses float64 = 0.0
	var NetIncome float64 = 0.0

	for i, v := range resampleTransactionTimeseries.dailyResample {
		resampledDatetime = append(resampledDatetime, i)

		resampledIncome = append(resampledIncome, v.income)
		TotalIncome += v.income
		TotalExpenses += v.expenses

		resampledExpense = append(resampledExpense, v.expenses)

		resampledBalance = append(resampledBalance, v.balance)

	}

	NetIncome = TotalIncome - TotalExpenses

	// Converting slices to JSON to pass to template:
	DatetimeJSON, err := json.Marshal(resampledDatetime)
	if err != nil {
		log.Fatal(err)
	}
	IncomeJSON, err := json.Marshal(resampledIncome)
	if err != nil {
		log.Fatal(err)
	}
	ExpenseJSON, err := json.Marshal(resampledExpense)
	if err != nil {
		log.Fatal(err)
	}
	BalanceJSON, err := json.Marshal(resampledBalance)
	if err != nil {
		log.Fatal(err)
	}

	data := struct {
		Transactions                          []Transaction
		DatetimeJSON                          string
		IncomeJSON                            string
		ExpenseJSON                           string
		BalanceJSON                           string
		TotalIncome, TotalExpenses, NetIncome string
	}{
		Transactions:  transactions,
		DatetimeJSON:  string(DatetimeJSON),
		IncomeJSON:    string(IncomeJSON),
		ExpenseJSON:   string(ExpenseJSON),
		BalanceJSON:   string(BalanceJSON),
		TotalIncome:   fmt.Sprintf("%.2f", TotalIncome),
		TotalExpenses: fmt.Sprintf("%.2f", TotalExpenses),
		NetIncome:     fmt.Sprintf("%.f", NetIncome),
	}

	fmt.Println(resampleTransactionTimeseries)

	tmpl, err := template.ParseFiles("../templates/index.html")
	if err != nil {
		log.Fatal("Unable to load the index.html template: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, data)
	if err != nil {

		if strings.Contains(err.Error(), "broken pipe") {
			log.Println("Client closed the connection prematurely")
			return
		}

		log.Fatal("Unable to render the index.html template: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleUpload(w http.ResponseWriter, r *http.Request) {

	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("../templates/upload.html")
		if err != nil {
			log.Fatal("Unable to render the csv upload template", err)
		}

		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Fatal("Error in executing the upload.html template")
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if r.Method == "POST" {
		// TODO: Extract and upload all transactions from the csv:

		dbPath := "./finance_database.sqlite"
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatal(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		file, header, err := r.FormFile("csvFile")
		if err != nil {
			log.Fatal("Error in Uploading the CSV File", err)
			return
		}
		defer file.Close()

		reader := csv.NewReader(file)
		records, err := reader.ReadAll()
		if err != nil {
			log.Fatal("Unable to read rows from the csv:", err)

		}

		tx, err := db.Begin()
		if err != nil {
			log.Fatal("Error in connecting to a database:", err)
		}
		stmt, err := tx.Prepare(`INSERT INTO transactions(
			unique_id, 
			date, 
			description, 
			debit, 
			credit
			) values(?, ?, ?, ? , ?)`)
		if err != nil {
			log.Fatal("Error in constructing transactions insert query", err)
		}
		defer stmt.Close()

		for i := 0; i < len(records); i++ {

			// Get a unique MD5 Hash for all elements for each row in the csv:
			var rawDate, rawDescription, rawDebit, rawCredit string
			record := records[i]
			rawDate = record[0]
			rawDescription = record[1]
			rawDebit = record[2]
			rawCredit = record[3]

			// If the credit or debit value is a blank string we set it to 0.0:
			h := md5.New()
			io.WriteString(h, rawDate)
			io.WriteString(h, rawDescription)
			io.WriteString(h, rawDebit)
			io.WriteString(h, rawCredit)

			// The last value appended to the array of csv rows is an MD5 hash for al existing records:
			transactionHash := hex.EncodeToString((h.Sum(nil)))

			_, err = stmt.Exec(transactionHash, record[0], record[1], record[2], record[3])
			if err != nil {
				log.Fatal("Unable to insert a specific test transaction row into db:", err)
			}
		}

		err = tx.Commit()
		if err != nil {
			log.Fatal("Unable to execute full insert query for test data:", err)
		}

		// Inserting tracking record for the uploaded csv:
		tx, err = db.Begin()
		if err != nil {
			log.Fatal("Error in connecting to a database:", err)
		}
		stmt, err = tx.Prepare(`INSERT INTO uploaded_files(
			filename,
			date_uploaded,
			num_rows,
			file_size
			) values(?, ?, ?, ?)`)
		if err != nil {
			log.Fatal("Error in constructing tracking insert query", err)
		}
		defer stmt.Close()

		today := time.Now()
		uploadedTime := today.Format("2006-01-02 15:04:05")
		_, err = stmt.Exec(header.Filename, uploadedTime, len(records), header.Size)
		if err != nil {
			log.Fatal("Unable to execute the insert query for the tracking record", err)
		}

		err = tx.Commit()
		if err != nil {
			log.Fatal("Unable to execute full insert query for transaction data:", err)
		}

		fmt.Println("Sucessfully Inserted all data into db.")

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func uploadHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {

		dbPath := "./finance_database.sqlite"
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatal(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		transactionHistory, err := ReadTransactionHistory(db)
		fmt.Println(transactionHistory)
		if err != nil {
			log.Fatal("Error in querying all of the transactions from the database")
		}

		tmpl, err := template.ParseFiles("../templates/upload_history.html")
		if err != nil {
			log.Fatal("Unable to render the upload history template", err)
		}

		err = tmpl.Execute(w, transactionHistory)
		if err != nil {
			log.Fatal("Unable to render the template snippit transaction history: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}
}

type uploadedCSVRecord struct {
	Date        string
	Description string
	Debit       string
	Credit      string
}

type rawUploadedCSVTableContent struct {
	FileName   string
	FileSize   int64
	NumRecords int
	CsvRecords []uploadedCSVRecord
}

type ErrorMessage struct {
	Error string
}

func displayUploadedCSVTable(w http.ResponseWriter, r *http.Request) {

	tmpl, err := template.ParseFiles("../templates/snippits/uploadedCsvTable.html")
	if err != nil {
		log.Fatal("Error in loading the template snippit: ", err)
	}

	if r.Method == "POST" {

		// Extracting the file content from the form:
		file, handler, err := r.FormFile("csvFile")
		if err != nil {
			log.Fatal("Error in Uploading the CSV File", err)
			return
		}
		defer file.Close()

		if !strings.Contains(handler.Filename, ".csv") {
			fmt.Println("A non csv file has been uploaded.")
			tmpl.ExecuteTemplate(w, "ErrorComponent", ErrorMessage{
				Error: "File Uploaded Not a CSV.",
			})
			return
		}

		fmt.Printf("Uploaded File: %+v\n", handler.Filename)
		fmt.Printf("File Size: %+v\n", handler.Size)
		fmt.Printf("MIME Header: %+v\n", handler.Header)

		fileContent := make([]byte, handler.Size)
		_, err = io.ReadFull(file, fileContent)
		if err != nil {
			log.Fatal("Unable to load the file content from the uploaded file")
		}

		// Loaded file content into a csv file:
		r := csv.NewReader(strings.NewReader(string(fileContent)))
		records, err := r.ReadAll()
		if err != nil {
			log.Fatal("Unable to parse the csv Reader:", err)
		}

		// Rendering the html table as a csv:
		uploadedTransactions := []uploadedCSVRecord{}
		for i := 0; i < len(records); i++ {

			uploadedTransactions = append(uploadedTransactions, uploadedCSVRecord{
				Date:        records[i][0],
				Description: records[i][1],
				Debit:       records[i][2],
				Credit:      records[i][3],
			})
		}

		uploadedCsvContent := rawUploadedCSVTableContent{
			FileName:   handler.Filename,
			FileSize:   handler.Size,
			NumRecords: len(records),
			CsvRecords: uploadedTransactions,
		}

		err = tmpl.ExecuteTemplate(w, "uploadedCsvTable", uploadedCsvContent)
		if err != nil {
			log.Fatal("Unable to render the template snippit for an uploaded csv file: ", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}

}

func displayTransactionContainer(w http.ResponseWriter, r *http.Request) {

	params := r.URL.Query()
	var transactionId string = params.Get("transaction_id")
	fmt.Println(transactionId)

	// Querying the transaction based on the ID
	dbPath := "./finance_database.sqlite"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	rows := db.QueryRow("SELECT * FROM transactions WHERE unique_id = ?", transactionId)
	if err != nil {
		log.Fatal("Error in querying a single transaction from the database:", err)
	}
	defer db.Close()

	var extractedUniqueId, extractedDate, extractedDescription, extractedDebit, extractedCredit string
	err = rows.Scan(&extractedUniqueId, &extractedDate, &extractedDescription, &extractedDebit, &extractedCredit)
	if err != nil {
		log.Fatal("Error in querying row from test database:", err)
	}

	var formattedCredit float32
	if extractedCredit == "" {
		formattedCredit = 0.0
	} else {
		formattedCredit64, err := strconv.ParseFloat(extractedCredit, 32)
		formattedCredit = float32(formattedCredit64)
		if err != nil {
			log.Fatal("Error in converting the credit value to a format: ", err)
		}

	}
	var formattedDebit float32
	if extractedDebit == "" {
		formattedDebit = 0.0
	} else {
		formattedDebit64, err := strconv.ParseFloat(extractedDebit, 32)
		formattedDebit = float32(formattedDebit64)
		if err != nil {
			log.Fatal("Error in converting the debit value to a format: ", err)
		}
	}

	transactionTime, err := time.Parse("2006-01-02", extractedDate)
	if err != nil {
		log.Fatal("Error in loading transaction time into a date struct", err)
	}

	individualTransaction := Transaction{
		UniqueId:    extractedUniqueId,
		Date:        transactionTime,
		Description: extractedDescription,
		Debit:       formattedDebit,
		Credit:      formattedCredit,
	}

	tmpl, err := template.ParseFiles("../templates/snippits/transactionInformationComponents.html")
	if err != nil {
		log.Fatal("Error in loading the template snippit: ", err)
	}
	err = tmpl.Execute(w, individualTransaction)
	if err != nil {
		log.Fatal("Unable to render the template snippit for an individual transaction: ", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println(individualTransaction)
}

func main() {

	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/upload_history", uploadHistoryHandler)

	// HTMX functions:
	http.HandleFunc("/get_transactions", displayTransactionContainer)
	http.HandleFunc("/render_csv", displayUploadedCSVTable)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("../css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("../js"))))

	log.Fatal(http.ListenAndServe(":8080", nil))
}
