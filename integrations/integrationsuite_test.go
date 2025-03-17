package integrations

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	_ "github.com/lib/pq"

	"github.com/pressly/goose/v3"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
)

type integrationSuite struct {
	suite.Suite
}

func (suite *integrationSuite) SetupSuite() {
	cfg := config.LoadConfig()

	var err error
	db, err = sql.Open("postgres", cfg.DSN)
	if err != nil {
		suite.T().Fatalf("sql.Open error: %v", err)
	}
	if err = db.Ping(); err != nil {
		suite.T().Fatalf("db.Ping error: %v", err)
	}

	if err := goose.Up(db, "../migrations"); err != nil {
		suite.T().Fatalf("goose.Up error: %v", err)
	}

	repo := repository.NewOrderRepository(db)
	srv := server.NewServer(repo, cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	testServer = httptest.NewServer(mux)

	if _, err := db.Exec("TRUNCATE orders CASCADE"); err != nil {
		suite.T().Logf("truncate error: %v", err)
	}
}

func (suite *integrationSuite) TearDownSuite() {
	testServer.Close()
	_ = db.Close()
}

func (suite *integrationSuite) TestCreateOrder() {
	order := models.Order{
		ID:              "test-integration-create-1",
		RecipientID:     "user001",
		StorageDeadline: time.Now().Add(2 * time.Hour),
		Weight:          5,
		Cost:            99,
	}

	resp, body := suite.doRequest(http.MethodPost, "/orders", order)

	assert.Equal(suite.T(), http.StatusCreated, resp.StatusCode)

	var got models.Order
	err := json.Unmarshal(body, &got)
	suite.Require().NoError(err, "json.Unmarshal error")
	suite.Assert().Equal(order.ID, got.ID)
}

func (suite *integrationSuite) TestListOrders() {
	for i := 1; i <= 2; i++ {
		order := models.Order{
			ID:              "test-list-" + strconv.Itoa(i),
			RecipientID:     "userL" + strconv.Itoa(i),
			StorageDeadline: time.Now().Add(2 * time.Hour),
		}
		suite.doRequest(http.MethodPost, "/orders", order)
	}

	resp, body := suite.doRequest(http.MethodGet, "/orders?cursor=&limit=10", nil)
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var orders []models.Order
	err := json.Unmarshal(body, &orders)
	suite.Require().NoError(err)
	suite.Assert().GreaterOrEqual(len(orders), 2)
}

func (suite *integrationSuite) TestUpdateOrder_NotFound() {
	update := models.Order{
		ID:              "unknown-id",
		StorageDeadline: time.Now().Add(2 * time.Hour),
	}
	resp, body := suite.doRequest(http.MethodPut, "/orders/unknown-id", update)
	suite.Assert().Equal(http.StatusInternalServerError, resp.StatusCode)

	suite.T().Log("Response body:", string(body))
}

func (suite *integrationSuite) TestDeleteOrder_NotFound() {
	resp, body := suite.doRequest(http.MethodDelete, "/orders/not-existing", nil)
	suite.Assert().Equal(http.StatusNotFound, resp.StatusCode)
	suite.T().Log("Response body:", string(body))
}

func (suite *integrationSuite) doRequest(method, path string, body any) (*http.Response, []byte) {
	var reqBody []byte
	var err error
	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			suite.T().Fatalf("json.Marshal error: %v", err)
		}
	}

	req, err := http.NewRequest(method, testServer.URL+path, bytes.NewReader(reqBody))
	if err != nil {
		suite.T().Fatalf("http.NewRequest: %v", err)
	}
	req.SetBasicAuth(testUsername, testPassword)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		suite.T().Fatalf("client.Do: %v", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		suite.T().Fatalf("ReadAll: %v", err)
	}
	return resp, respBody
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(integrationSuite))
}
