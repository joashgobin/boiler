package payments

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/joashgobin/boiler/helpers"
)

// Environment represents a Postman environment file
type Environment struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	Values               []EnvironmentVariable `json:"values"`
	PostmanVariableScope string                `json:"_postman_variable_scope"`
	PostmanExportedAt    string                `json:"_postman_exported_at"`
	PostmanExportedUsing string                `json:"_postman_exported_using"`
}

// EnvironmentVariable represents a single environment variable
type EnvironmentVariable struct {
	Key     string  `json:"key"`
	Value   string  `json:"value"`
	Type    *string `json:"type"` // Pointer to handle optional field
	Enabled bool    `json:"enabled"`
}

// Party represents debit or credit party information
type Party struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Transaction represents a single transaction
type Transaction struct {
	Amount             string    `json:"amount"`
	Currency           string    `json:"currency"`
	DisplayType        string    `json:"displayType"`
	TransactionStatus  string    `json:"transactionStatus"`
	DescriptionText    string    `json:"descriptionText"`
	ModificationDate   time.Time `json:"modificationDate"`
	TransactionRef     string    `json:"transactionReference"`
	TransactionReceipt string    `json:"transactionReceipt"`
	DebitParty         []Party   `json:"debitParty"`
	CreditParty        []Party   `json:"creditParty"`
}

// Response represents the complete JSON structure
type TransactionsResponse struct {
	ExecutionID  string        `json:"executionId"`
	Transactions []Transaction `json:"TransactionList"`
}

type TransactionModel struct {
	DB *sql.DB
}

type MMGTransaction struct {
	Timestamp time.Time
	Reference string
	From      string
	To        string
	Amount    float64
	Currency  string
	Category  string
	Status    string
	Metadata  string
}

// Config holds application configuration
type Config struct {
	Merchant       string `json:"merchant"`
	MerchantMsisdn string `json:"merchant_msisdn"`
	SecretKey      string `json:"secret_key"`
	Amount         string `json:"amount"`
	ClientID       string `json:"client_id"`
}

// TokenParams represents token generation parameters
type TokenParams struct {
	SecretKey             string `json:"secretKey"`
	Amount                string `json:"amount"`
	MerchantID            string `json:"merchantId"`
	MerchantTransactionID string `json:"merchantTransactionId"`
	ProductDescription    string `json:"productDescription"`
	RequestInitiationTime int64  `json:"requestInitiationTime"`
	MerchantName          string `json:"merchantName"`
}

func getTransactionMeta(data []byte) (string, error) {
	r := regexp.MustCompile(`"key":\s*"(product_desc|description)",\s*"value":\s*"(.+?)"`)
	matches := r.FindStringSubmatch(string(data))
	if len(matches) > 1 {
		return matches[2], nil
	}
	return "", errors.New("can't find description in body")
}

func extractResourceTokenFromBody(data []byte) (string, error) {
	r := regexp.MustCompile(`"access_token":\s*"(.*?)"`)
	matches := r.FindStringSubmatch(string(data))
	if len(matches) > 0 {
		return matches[1], nil
	}
	return "", errors.New("can't find resource token in body")
}

func LoadMMGTransactionDetails(db *sql.DB, merchantNumber int, transactionReference string, resourceToken string) {
	helpers.Background(
		func() {
			var urlBuilder strings.Builder
			urlBuilder.WriteString("https://uat-api.mmg.gy/transactiondetails/")
			urlBuilder.WriteString(transactionReference)
			url := urlBuilder.String()
			fmt.Printf("Making request to: %s\n", url)
			method := "GET"

			payload := strings.NewReader("{\"query\":\"\",\"variables\":{}}")

			client := &http.Client{}
			req, err := http.NewRequest(method, url, payload)

			if err != nil {
				log.Error(err)
				return
			}
			// get merchant environment details
			pairs, err := getEnvironmentData(merchantNumber)
			if err != nil {
				return
			}
			req.Header.Add("x-wss-token", "Bearer "+resourceToken)
			req.Header.Add("x-wss-mid", pairs["merchant_mid"])
			req.Header.Add("x-wss-mkey", pairs["merchant_mkey"])
			req.Header.Add("x-wss-msecret", pairs["merchant_msecret"])
			req.Header.Add("x-wss-correlationid", helpers.GetRandomUUID())
			req.Header.Add("x-api-key", os.Getenv("MMG_API_KEY"))

			res, err := client.Do(req)
			if err != nil {
				log.Error(err)
				return
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				log.Error(err)
				return
			}
			// fmt.Println(string(body))
			metadata, err := getTransactionMeta(body)
			if err != nil {
				log.Errorf("error getting transaction meta: %v", err)
			} else {
				query := `
				UPDATE transactions
				SET metadata = ?
				WHERE reference = ?
				AND metadata IS NULL
				`
				result, err := db.Exec(query, metadata, transactionReference)
				if err != nil {
					log.Errorf("failed to update transaction: ref %d", transactionReference)
					return
				}
				rowsAffected, err := result.RowsAffected()
				if err != nil {
					log.Errorf("failed to check the affected rows: %v", err)
					return
				}
				if rowsAffected == 0 {
					log.Errorf("%v", sql.ErrNoRows)
					return
				}
				log.Infof("updated metadata for transaction: %s", transactionReference)
			}
		})
}

func getTransactionData(db *sql.DB, data []byte, merchantNumber int, resourceToken string) {
	var response TransactionsResponse
	err := json.Unmarshal(data, &response)
	if err != nil {
		log.Errorf("error unmarshaling JSON: %v\n", err)
		return
	}
	var history []MMGTransaction
	for _, transaction := range response.Transactions {
		var mmgTransaction MMGTransaction

		mmgTransaction.Amount, _ = strconv.ParseFloat(transaction.Amount, 64)
		mmgTransaction.Currency = transaction.Currency
		mmgTransaction.Status = transaction.TransactionStatus
		mmgTransaction.Category = transaction.DisplayType
		mmgTransaction.Reference = transaction.TransactionRef
		mmgTransaction.Timestamp = transaction.ModificationDate

		for _, party := range transaction.DebitParty {
			if party.Key == "accountid" {
				mmgTransaction.From = party.Value
			}
		}

		for _, party := range transaction.CreditParty {
			if party.Key == "accountid" {
				mmgTransaction.To = party.Value
			}
		}
		history = append(history, mmgTransaction)
	}

	// only metadata is not set
	stmt, err := db.Prepare(`
	INSERT INTO transactions (
		timestamp,
		reference,
		source,
		destination,
		amount,
		currency,
		category,
		status
	) VALUES (?,?,?,?,?,?,?,?)
	`)
	if err != nil {
		log.Errorf("prepare statement error: %v\n", err)
		return
	}
	defer stmt.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Errorf("begin transaction error: %v\n", err)
		return
	}

	for _, txn := range history {
		_, err := stmt.Exec(
			txn.Timestamp,
			txn.Reference,
			txn.From,
			txn.To,
			txn.Amount,
			txn.Currency,
			txn.Category,
			txn.Status,
		)
		if err != nil {
			tx.Rollback()
			log.Errorf("insert error: %v", err)
		} else {
			log.Infof("inserted %s) %s: %s -> %s (%f %s)\n", txn.Reference, txn.Category, txn.From, txn.To, txn.Amount, txn.Currency)
		}
	}
	tx.Commit()
	for _, txn := range history {
		LoadMMGTransactionDetails(db, merchantNumber, txn.Reference, resourceToken)
	}
}

func getEnvironmentData(merchantNumber int) (map[string]string, error) {
	data, err := os.ReadFile("merchants/" + strconv.Itoa(merchantNumber) + ".postman_environment")
	if err != nil {
		fmt.Printf("error reading file: %v\n", err)
		return nil, err
	}

	// unmarshal JSON
	var env Environment
	err = json.Unmarshal(data, &env)
	if err != nil {
		fmt.Printf("error parsing JSON: %v\n", err)
		return nil, err
	}

	var pairs = make(map[string]string)

	for _, value := range env.Values {
		pairs[value.Key] = value.Value
	}
	return pairs, nil
}

func getResourceToken(db *sql.DB, merchantNumber int) string {
	return helpers.GetShelf(db, "resource-token-"+strconv.Itoa(merchantNumber))
}

func IsMMGSubscribed(db *sql.DB, thresholdAmount float64, userEmail string) bool {
	log.Infof("checking for subscription for %s", userEmail)
	count := 0
	total := float64(0)
	query := `SELECT
	timestamp, reference, source, destination, amount, currency, category, status, metadata
	FROM transactions WHERE metadata COLLATE utf8mb4_bin LIKE ? AND amount >= ?`
	rows, err := db.Query(query, fmt.Sprintf("%%%s%%", userEmail), thresholdAmount)
	if err != nil {
		log.Errorf("query error: %v", err)
		return false
	}
	defer rows.Close()

	var transactions []MMGTransaction
	for rows.Next() {
		var txn MMGTransaction
		err := rows.Scan(
			&txn.Timestamp,
			&txn.Reference,
			&txn.From,
			&txn.To,
			&txn.Amount,
			&txn.Currency,
			&txn.Category,
			&txn.Status,
			&txn.Metadata,
		)
		if err != nil {
			log.Errorf("scan error: %v", err)
		}
		transactions = append(transactions, txn)
		total += txn.Amount
		count++
	}
	if err := rows.Err(); err != nil {
		log.Errorf("rows error: %v", err)
	}
	log.Infof("found %d subscription(s) for %s of at least $%.2f, total $%.2f", count, userEmail, thresholdAmount, total)
	return count > 0
}

func LoadMMGTransactionHistory(db *sql.DB, merchantNumber int) {
	helpers.Background(
		func() {
			now := time.Now()
			toDate := now.AddDate(0, 0, 0).Format("2006-01-02")
			fromDate := now.AddDate(0, 0, -30).Format("2006-01-02")

			var urlBuilder strings.Builder
			urlBuilder.WriteString("https://uat-api.mmg.gy/ministatement/")
			urlBuilder.WriteString(strconv.Itoa(merchantNumber))
			urlBuilder.WriteString("?offset=1")
			urlBuilder.WriteString("&fromdate=" + fromDate)
			urlBuilder.WriteString("&todate=" + toDate)
			url := urlBuilder.String()
			fmt.Printf("Making request to: %s\n", url)
			method := "GET"

			payload := strings.NewReader("{\"query\":\"\",\"variables\":{}}")

			client := &http.Client{}
			req, err := http.NewRequest(method, url, payload)

			if err != nil {
				log.Error(err)
				return
			}
			// get merchant environment details
			pairs, err := getEnvironmentData(merchantNumber)
			if err != nil {
				return
			}
			resourceToken := getResourceToken(db, merchantNumber)
			req.Header.Add("x-wss-token", "Bearer "+resourceToken)
			req.Header.Add("x-wss-mid", pairs["merchant_mid"])
			req.Header.Add("x-wss-mkey", pairs["merchant_mkey"])
			req.Header.Add("x-wss-msecret", pairs["merchant_msecret"])
			req.Header.Add("x-wss-correlationid", helpers.GetRandomUUID())
			req.Header.Add("x-api-key", os.Getenv("MMG_API_KEY"))
			req.Header.Add("Content-Type", "application/json")

			res, err := client.Do(req)
			if err != nil {
				log.Errorf("mmg history response error: %v", err)
				return
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				log.Error(err)
				return
			}

			// in case resource token is invalid
			if strings.Contains(string(body), "clientAuthorisationError") {
				log.Error("failed to use valid resource token")
				LoadNewResourceToken(db, merchantNumber)
				LoadMMGTransactionHistory(db, merchantNumber)
				return
			}

			getTransactionData(db, body, merchantNumber, resourceToken)
		})
}

func LoadNewResourceToken(db *sql.DB, merchantNumber int) {
	url := "https://gtt-uat-oauth2-service-api.qpass.com:9143/oauth2-endpoint/oauth/resourcetoken"
	method := "POST"
	var payloadBuilder strings.Builder
	payloadBuilder.WriteString("grant_type=password")
	payloadBuilder.WriteString("&api_key=" + os.Getenv("MMG_API_ALT"))
	payloadBuilder.WriteString("&username=" + strconv.Itoa(merchantNumber))
	payloadBuilder.WriteString("&password=" + os.Getenv("MMG_PASSWORD"))
	payload := strings.NewReader(payloadBuilder.String())
	// log.Infof("payload: %s", &payloadBuilder)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		fmt.Println(err)
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Cookie", "TS01e7487e=01d0cb83601844ab4044ae5123cd771d7be13b17ff6e839eb176bdd1b5105c71df1d40d5f9740a326566896199238d4c0a47f4bd4a")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(body))
	token, err := extractResourceTokenFromBody(body)

	if err != nil {
		LoadNewResourceToken(db, merchantNumber)
	}
	log.Infof("new resource token: %s", token)
	helpers.SetShelf(db, "resource-token-"+strconv.Itoa(merchantNumber), token)
}

func GetMMGBalance(merchantNumber int) {
	helpers.Background(
		func() {
			// build request url
			var urlBuilder strings.Builder
			urlBuilder.WriteString("https://uat-api.mmg.gy/balancecheck/")
			urlBuilder.WriteString(strconv.Itoa(merchantNumber))
			url := urlBuilder.String()
			fmt.Printf("Making request to: %s\n", url)

			// set the http method
			method := "GET"

			// initialize the http client
			client := &http.Client{}
			req, err := http.NewRequest(method, url, nil)
			if err != nil {
				fmt.Println(err)
				return
			}
			// get merchant environment details
			pairs, err := getEnvironmentData(merchantNumber)
			if err != nil {
				return
			}
			req.Header.Add("x-wss-token", "Bearer "+getResourceToken(nil, merchantNumber))
			req.Header.Add("x-wss-mid", pairs["merchant_mid"])
			req.Header.Add("x-wss-mkey", pairs["merchant_mkey"])
			req.Header.Add("x-wss-msecret", pairs["merchant_msecret"])
			req.Header.Add("x-wss-correlationid", helpers.GetRandomUUID())
			req.Header.Add("x-api-key", os.Getenv("MMG_API_KEY"))
			// for key, values := range req.Header {
			// fmt.Printf("%s: %v\n", key, values)
			// }

			// perform the request
			res, err := client.Do(req)
			if err != nil {
				fmt.Println(err)
				return
			}
			defer res.Body.Close()

			// output request body
			body, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(string(body))
		})
}

func FiberMMGSubscriptionMiddleware(db *sql.DB, store *session.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		username := "123"
		sess, err := store.Get(c)
		if err != nil {
			panic(err)
		}
		subscribed := sess.Get("mmg_subscribed")
		log.Infof("subscribed: %v", subscribed)
		// log.Infof("keys: %v", sess.Keys())
		if subscribed != "yes" {
			if !IsMMGSubscribed(db, 100, username) {
				log.Infof("subscription not found for %s, redirecting to home", username)
				return c.Redirect("/")
			}
		}
		sess.Set("mmg_subscribed", "yes")
		if err := sess.Save(); err != nil {
			panic(err)
		}
		return c.Next()
	}
}

func loadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	config := &Config{}
	inDefaultSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if line == "[DEFAULT]" {
				inDefaultSection = true
			} else {
				inDefaultSection = false
			}
			continue
		}

		// Only process key-value pairs in DEFAULT section
		if !inDefaultSection {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "merchant":
			config.Merchant = value
		case "merchant_msisdn":
			config.MerchantMsisdn = value
		case "secret_key":
			config.SecretKey = value
		case "amount":
			config.Amount = value
		case "client_id":
			config.ClientID = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Validate required fields
	/*
		if config.Merchant == "" || config.MerchantMsisdn == "" ||
			config.SecretKey == "" || config.Amount == "" || config.ClientID == "" {
			return nil, fmt.Errorf("missing required configuration fields")
		}
	*/
	fmt.Printf("config: %v", config)

	return config, nil
}

func loadPrivateKey(filename string) (*rsa.PrivateKey, error) {
	privateKeyData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM data found in key file")
	}

	// Use PKCS#8 parser
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaPriv, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA private key")
	}

	return rsaPriv, nil
}

func loadPublicKey(filename string) (*rsa.PublicKey, error) {
	publicKeyData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(publicKeyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM data found in key file")
	}

	// Try PKIX public key format first
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// If that fails, try PKCS#1 public key format
		if parsedKey, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
			return parsedKey, nil
		}
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA public key")
	}

	return rsaPub, nil
}

func generateURL(token []byte, msisdn, clientID string) string {
	tokenStr := base64.URLEncoding.EncodeToString(token)
	fmt.Printf("-- CHECKOUT URL PARAMS --\n")
	fmt.Printf("MSISDN: %s\n", msisdn)
	fmt.Printf("CLIENTID: %s\n", clientID)
	fmt.Printf("TOKEN: %s\n\n", tokenStr)

	fmt.Printf("-- CHECKOUT URL --\n")
	return fmt.Sprintf("https://gtt-uat-checkout.qpass.com:8743/checkout-endpoint/home?token=%s&merchantId=%s&X-Client-ID=%s\n",
		tokenStr, msisdn, clientID)
}

func encrypt(data interface{}, publicKey *rsa.PublicKey) ([]byte, error) {
	jsonData, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}
	log.Infof("Checkout Object:\n%s\n", jsonData)

	hash := sha256.New()
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, []byte(jsonData), nil)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	return ciphertext, nil
}

func decrypt(ciphertext []byte, privateKey *rsa.PrivateKey) (map[string]interface{}, error) {
	hash := sha256.New()
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(plaintext, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted data: %w", err)
	}

	log.Infof("Decrypted response:", data)
	return data, nil
}

func InitiateCheckout(merchantNumber int, merchantName, itemDescription string) string {
	config, err := loadConfig(fmt.Sprintf("merchants/%d.cfg", merchantNumber))
	if err != nil {
		log.Fatal(err)
	}

	publicKey, err := loadPublicKey(fmt.Sprintf("merchants/%s-keys/%s.public.pem", config.MerchantMsisdn, config.MerchantMsisdn))
	if err != nil {
		log.Fatal(err)
	}

	timestamp := time.Now().Unix()
	tokenParams := TokenParams{
		SecretKey:             config.SecretKey,
		Amount:                config.Amount,
		MerchantID:            config.MerchantMsisdn,
		MerchantTransactionID: fmt.Sprint(timestamp),
		ProductDescription:    itemDescription,
		RequestInitiationTime: timestamp,
		MerchantName:          merchantName,
	}

	token, err := encrypt(tokenParams, publicKey)
	if err != nil {
		log.Fatal(err)
	}

	return generateURL(token, config.MerchantMsisdn, config.ClientID)
}

func InitMMG(db *sql.DB, appName string) {
	helpers.RunMigration(strings.ReplaceAll(`
-- Select database
USE <appName>;

-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER NOT NULL PRIMARY KEY AUTO_INCREMENT,
    timestamp DATETIME NOT NULL,
        reference VARCHAR(20) NOT NULL UNIQUE,
        source      VARCHAR(20) NOT NULL,
        destination        VARCHAR(20) NOT NULL,
        amount    DECIMAL(10,2) NOT NULL,
        currency  VARCHAR(5) NOT NULL,
        category  VARCHAR(30) NOT NULL,
        status    VARCHAR(20) NOT NULL,
    metadata VARCHAR(100)
);
	`, "<appName>", appName), db)
}
