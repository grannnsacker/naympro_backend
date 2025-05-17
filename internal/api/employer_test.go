package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aalug/job-finder-go/internal/db/mock"
	db "github.com/aalug/job-finder-go/internal/db/sqlc"
	"github.com/aalug/job-finder-go/internal/worker"
	mockworker "github.com/aalug/job-finder-go/internal/worker/mock"
	"github.com/aalug/job-finder-go/pkg/token"
	"github.com/aalug/job-finder-go/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang/mock/gomock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type eqCreateEmployerTxParamsMatcher struct {
	arg      db.CreateEmployerTxParams
	password string
	employer db.Employer
}

func (e eqCreateEmployerTxParamsMatcher) Matches(x interface{}) bool {
	actualArg, ok := x.(db.CreateEmployerTxParams)
	if !ok {
		return false
	}

	err := utils.CheckPassword(e.password, actualArg.HashedPassword)
	if err != nil {
		return false
	}

	e.arg.HashedPassword = actualArg.HashedPassword
	if !reflect.DeepEqual(e.arg.CreateEmployerParams, actualArg.CreateEmployerParams) {
		return false
	}

	err = actualArg.AfterCreate(e.employer)
	return err == nil
}

func (e eqCreateEmployerTxParamsMatcher) String() string {
	return fmt.Sprintf("matches arg %v and password %v", e.arg, e.password)
}

func EqCreateEmployerTxParams(arg db.CreateEmployerTxParams, password string, employer db.Employer) gomock.Matcher {
	return eqCreateEmployerTxParamsMatcher{arg, password, employer}
}

func TestCreateEmployerAPI(t *testing.T) {
	employer, password, company := generateRandomEmployerAndCompany(t)

	requestBody := gin.H{
		"email":            employer.Email,
		"full_name":        employer.FullName,
		"password":         password,
		"company_name":     company.Name,
		"company_industry": company.Industry,
		"company_location": company.Location,
	}

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				companyParams := db.CreateCompanyParams{
					Name:     company.Name,
					Industry: company.Industry,
					Location: company.Location,
				}
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Eq(companyParams)).
					Times(1).
					Return(company, nil)
				employerParams := db.CreateEmployerTxParams{
					CreateEmployerParams: db.CreateEmployerParams{
						CompanyID:      company.ID,
						FullName:       employer.FullName,
						Email:          employer.Email,
						HashedPassword: employer.HashedPassword,
					},
				}
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), EqCreateEmployerTxParams(employerParams, password, employer)).
					Times(1).
					Return(db.CreateEmployerTxResult{
						Employer: employer,
					}, nil)
				taskPayload := &worker.PayloadSendVerificationEmail{
					Email: employer.Email,
				}
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), taskPayload, gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				requireBodyMatchEmployerAndCompany(t, recorder.Body, employer, company)
			},
		},
		{
			name: "Invalid Request Body",
			body: gin.H{
				"email":     "invalid_email",
				"full_name": "full name",
				"password":  "password",
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Company Name Already Exists",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, &pq.Error{Code: "23505"})
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Internal Server Error CreateCompany",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Employer Email Already Exists",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateEmployerTxResult{}, &pq.Error{Code: "23505"})
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "Internal Server Error CreateEmployerTx",
			body: requestBody,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					CreateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					CreateEmployerTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateEmployerTxResult{}, sql.ErrConnDone)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockdb.NewMockStore(ctrl)

			taskCtrl := gomock.NewController(t)
			defer taskCtrl.Finish()
			taskDistributor := mockworker.NewMockTaskDistributor(taskCtrl)

			tc.buildStubs(store, taskDistributor)

			server := newTestServer(t, store, nil, taskDistributor)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := BaseUrl + "/employers"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestLoginEmployerAPI(t *testing.T) {
	employer, password, company := generateRandomEmployerAndCompany(t)
	employer.IsEmailVerified = true

	testCases := []struct {
		name          string
		body          gin.H
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Employer Not Found",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetCompanyByID",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Company Not Found",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Invalid Email",
			body: gin.H{
				"email":    "invalid",
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Incorrect Password",
			body: gin.H{
				"email":    employer.Email,
				"password": fmt.Sprintf("%d, %s", utils.RandomInt(1, 1000), utils.RandomString(10)),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Password Too Short",
			body: gin.H{
				"email":    employer.Email,
				"password": "abc",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Email Not Verified",
			body: gin.H{
				"email":    employer.Email,
				"password": password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				employer.IsEmailVerified = false
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := BaseUrl + "/employers/login"
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestGetEmployerAPI(t *testing.T) {
	employer, _, company := generateRandomEmployerAndCompany(t)
	user, _ := generateRandomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, r *http.Request, maker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchEmployerAndCompany(t, recorder.Body, employer, company)
			},
		},
		{
			name: "Unauthorized Only Employer Access",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, user.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetCompanyByID",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers"
			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, req, server.tokenMaker)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestUpdateEmployerAPI(t *testing.T) {
	employer, _, company := generateRandomEmployerAndCompany(t)
	newEmployer, _, newCompany := generateRandomEmployerAndCompany(t)
	user, _ := generateRandomUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, r *http.Request, maker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"full_name":        newEmployer.FullName,
				"email":            newEmployer.Email,
				"company_name":     newCompany.Name,
				"company_industry": newCompany.Industry,
				"company_location": newCompany.Location,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
				companyParams := db.UpdateCompanyParams{
					ID:       company.ID,
					Name:     newCompany.Name,
					Industry: newCompany.Industry,
					Location: newCompany.Location,
				}
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Eq(companyParams)).
					Times(1).
					Return(newCompany, nil)
				employerParams := db.UpdateEmployerParams{
					ID:        employer.ID,
					CompanyID: employer.CompanyID,
					FullName:  newEmployer.FullName,
					Email:     newEmployer.Email,
				}
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Eq(employerParams)).
					Times(1).
					Return(newEmployer, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchEmployerAndCompany(t, recorder.Body, newEmployer, newCompany)
			},
		},
		{
			name: "Unauthorized Only Employer Access",
			body: gin.H{
				"email":            newEmployer.Email,
				"company_industry": newCompany.Industry,
				"company_location": newCompany.Location,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, user.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			body: gin.H{
				"email":            newEmployer.Email,
				"company_industry": newCompany.Industry,
				"company_location": newCompany.Location,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetCompanyByID",
			body: gin.H{
				"full_name":        newEmployer.FullName,
				"company_location": newCompany.Location,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error UpdateCompany",
			body: gin.H{
				"full_name":    newEmployer.FullName,
				"company_name": newCompany.Name,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Company{}, sql.ErrConnDone)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error UpdateEmployer",
			body: gin.H{
				"company_name": newCompany.Name,
				"full_name":    newEmployer.FullName,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Eq(employer.CompanyID)).
					Times(1).
					Return(company, nil)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(newCompany, nil)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "invalid Body",
			body: gin.H{
				"company_name": 123,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "invalid Email",
			body: gin.H{
				"email": "invalid",
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetCompanyByID(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := BaseUrl + "/employers"
			req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, req, server.tokenMaker)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestUpdateEmployerPasswordAPI(t *testing.T) {
	employer, password, _ := generateRandomEmployerAndCompany(t)
	newPassword := utils.RandomString(6)
	user, _ := generateRandomUser(t)

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, r *http.Request, maker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"old_password": password,
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Old Password Too Short",
			body: gin.H{
				"old_password": "123",
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "New Password Too Short",
			body: gin.H{
				"old_password": password,
				"new_password": "123",
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Incorrect Password",
			body: gin.H{
				"old_password": "incorrect",
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Internal Server Error UpdateEmployerPassword",
			body: gin.H{
				"old_password": password,
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(1).
					Return(sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Unauthorized Only Employer Access",
			body: gin.H{
				"old_password": password,
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, user.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			body: gin.H{
				"old_password": password,
				"new_password": newPassword,
			},
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					UpdateEmployerPassword(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := BaseUrl + "/employers/password"
			req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, req, server.tokenMaker)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestDeleteEmployerAPI(t *testing.T) {
	employer, _, company := generateRandomEmployerAndCompany(t)
	user, _ := generateRandomUser(t)

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, r *http.Request, maker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteCompany(gomock.Any(), gomock.Eq(company.ID)).
					Times(1).
					Return(nil)
				store.EXPECT().
					DeleteEmployer(gomock.Any(), gomock.Eq(employer.ID)).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNoContent, recorder.Code)
			},
		},
		{
			name: "Unauthorized Only Employer Access",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, user.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					DeleteCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					DeleteEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "Internal Server Error GetEmployerByEmail",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					DeleteCompany(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					DeleteEmployer(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error DeleteCompany",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(sql.ErrConnDone)
				store.EXPECT().
					DeleteEmployer(gomock.Any(), gomock.Eq(employer.ID)).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Internal Server Error DeleteEmployer",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteCompany(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
				store.EXPECT().
					DeleteEmployer(gomock.Any(), gomock.Eq(employer.ID)).
					Times(1).
					Return(sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers"
			req, err := http.NewRequest(http.MethodDelete, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, req, server.tokenMaker)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestGetUserAsEmployerAPI(t *testing.T) {
	employer, _, _ := generateRandomEmployerAndCompany(t)
	user, _ := generateRandomUser(t)
	var userSkills []db.UserSkill
	for i := 0; i < 5; i++ {
		userSkills = append(userSkills, db.UserSkill{
			ID:         int32(i),
			UserID:     user.ID,
			Skill:      utils.RandomString(4),
			Experience: utils.RandomInt(1, 5),
		})
	}

	testCases := []struct {
		name          string
		userEmail     string
		setupAuth     func(t *testing.T, r *http.Request, maker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(user, userSkills, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user, userSkills)
			},
		},
		{
			name:      "Invalid Email",
			userEmail: "invalid",
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "Unauthorized Only Employer Access",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, user.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name:      "Internal Server Error GetEmployerByEmail",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:      "Internal Server Error GetEmployerByEmail",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:      "User Not Found",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.User{}, []db.UserSkill{}, sql.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "Internal Server Error GetUserDetailsByEmail",
			userEmail: user.Email,
			setupAuth: func(t *testing.T, r *http.Request, maker token.Maker) {
				addAuthorization(t, r, maker, authorizationTypeBearer, employer.Email, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					GetUserDetailsByEmail(gomock.Any(), gomock.Eq(user.Email)).
					Times(1).
					Return(db.User{}, []db.UserSkill{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers/user-details/" + tc.userEmail
			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, req, server.tokenMaker)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestGetEmployerAndCompanyDetailsAPI(t *testing.T) {
	employer, _, company := generateRandomEmployerAndCompany(t)
	details := db.GetEmployerAndCompanyDetailsRow{
		CompanyName:      company.Name,
		CompanyIndustry:  company.Industry,
		CompanyLocation:  company.Location,
		CompanyID:        company.ID,
		EmployerID:       employer.ID,
		EmployerFullName: employer.FullName,
		EmployerEmail:    employer.Email,
	}

	testCases := []struct {
		name          string
		employerEmail string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			employerEmail: employer.Email,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerAndCompanyDetails(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(details, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchEmployerAndCompanyDetails(t, recorder.Body, details)
			},
		},
		{
			name:          "Invalid Email",
			employerEmail: "invalid",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerAndCompanyDetails(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:          "Employer Not Found",
			employerEmail: employer.Email,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerAndCompanyDetails(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.GetEmployerAndCompanyDetailsRow{}, sql.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:          "Internal Server Error",
			employerEmail: employer.Email,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetEmployerAndCompanyDetails(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.GetEmployerAndCompanyDetailsRow{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers/employer-company-details/" + tc.employerEmail
			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestVerifyEmployerEmailAPI(t *testing.T) {
	employer, _, _ := generateRandomEmployerAndCompany(t)
	verifyEmail := db.VerifyEmail{
		ID:         int64(utils.RandomInt(1, 1000)),
		Email:      employer.Email,
		SecretCode: utils.RandomString(32),
		IsUsed:     false,
		CreatedAt:  time.Now(),
		ExpiredAt:  time.Now().Add(15 * time.Minute),
	}

	type Query struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}

	testCases := []struct {
		name          string
		query         Query
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			query: Query{
				ID:   verifyEmail.ID,
				Code: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				params := db.VerifyEmailTxParams{
					ID:         verifyEmail.ID,
					SecretCode: verifyEmail.SecretCode,
				}
				verifyEmail.IsUsed = true
				employer.IsEmailVerified = true
				store.EXPECT().
					VerifyEmployerEmailTx(gomock.Any(), gomock.Eq(params)).
					Times(1).
					Return(db.VerifyEmployerEmailResult{
						Employer:    employer,
						VerifyEmail: verifyEmail,
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "Internal Server Error",
			query: Query{
				ID:   verifyEmail.ID,
				Code: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyEmployerEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyEmployerEmailResult{}, sql.ErrConnDone)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "Verify Email Not Found",
			query: Query{
				ID:   verifyEmail.ID,
				Code: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyEmployerEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyEmployerEmailResult{}, sql.ErrNoRows)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "Invalid Code Length",
			query: Query{
				ID:   verifyEmail.ID,
				Code: utils.RandomString(31),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyEmployerEmailTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "Invalid ID",
			query: Query{
				ID:   0,
				Code: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyEmployerEmailTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store, nil, nil)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers/verify-email"

			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			q := req.URL.Query()
			q.Add("id", fmt.Sprintf("%d", tc.query.ID))
			q.Add("code", tc.query.Code)
			req.URL.RawQuery = q.Encode()

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

func TestSendVerificationEmailToEmployerAPI(t *testing.T) {
	employer, _, _ := generateRandomEmployerAndCompany(t)
	testCases := []struct {
		name          string
		email         string
		buildStubs    func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(nil)
				taskPayload := &worker.PayloadSendVerificationEmail{
					Email: employer.Email,
				}
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Eq(taskPayload), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "Invalid Email",
			email: "invalid",
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Any()).
					Times(0)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "Employer Not Found",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrNoRows)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:  "Internal Server Error GetEmployerByEmail",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(db.Employer{}, sql.ErrConnDone)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Any()).
					Times(0)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:  "DeleteVerifyEmail ErrNoRows Do Nothing",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(sql.ErrNoRows)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "Internal Server Error DeleteVerifyEmail",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(sql.ErrConnDone)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:  "Internal Server Error DistributeTaskSendVerificationEmail",
			email: employer.Email,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				store.EXPECT().
					GetEmployerByEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(employer, nil)
				store.EXPECT().
					DeleteVerifyEmail(gomock.Any(), gomock.Eq(employer.Email)).
					Times(1).
					Return(nil)
				distributor.EXPECT().
					DistributeTaskSendVerificationEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("some error"))
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}
	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockdb.NewMockStore(ctrl)

			taskCtrl := gomock.NewController(t)
			defer taskCtrl.Finish()
			taskDistributor := mockworker.NewMockTaskDistributor(taskCtrl)

			tc.buildStubs(store, taskDistributor)

			server := newTestServer(t, store, nil, taskDistributor)
			recorder := httptest.NewRecorder()

			url := BaseUrl + "/employers/send-verification-email"

			req, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			q := req.URL.Query()
			q.Add("email", tc.email)
			req.URL.RawQuery = q.Encode()

			server.router.ServeHTTP(recorder, req)

			tc.checkResponse(recorder)
		})
	}
}

// generateRandomEmployer create a random employer and company
func generateRandomEmployerAndCompany(t *testing.T) (db.Employer, string, db.Company) {
	password := utils.RandomString(6)
	hashedPassword, err := utils.HashPassword(password)
	require.NoError(t, err)

	company := db.Company{
		ID:       utils.RandomInt(1, 100),
		Name:     utils.RandomString(5),
		Industry: utils.RandomString(5),
		Location: utils.RandomString(6),
	}

	employer := db.Employer{
		ID:             utils.RandomInt(1, 100),
		CompanyID:      company.ID,
		FullName:       utils.RandomString(5),
		Email:          utils.RandomEmail(),
		HashedPassword: hashedPassword,
		CreatedAt:      time.Now(),
	}

	return employer, password, company
}

// requireBodyMatchEmployerAndCompany checks if the body of the response matches the employer and company
func requireBodyMatchEmployerAndCompany(t *testing.T, body *bytes.Buffer, employer db.Employer, company db.Company) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response employerResponse
	err = json.Unmarshal(data, &response)

	require.NoError(t, err)
	require.NotZero(t, response.EmployerID)
	require.Equal(t, employer.Email, response.Email)
	require.Equal(t, employer.FullName, response.FullName)
	require.Equal(t, employer.CompanyID, response.CompanyID)
	require.Equal(t, company.Name, response.CompanyName)
	require.Equal(t, company.Industry, response.CompanyIndustry)
	require.Equal(t, company.Location, response.CompanyLocation)
	require.WithinDuration(t, employer.CreatedAt, response.EmployerCreatedAt, time.Second)
}

func requireBodyMatchEmployerAndCompanyDetails(t *testing.T, body *bytes.Buffer, details db.GetEmployerAndCompanyDetailsRow) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response db.GetEmployerAndCompanyDetailsRow
	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	require.Equal(t, details.CompanyName, response.CompanyName)
	require.Equal(t, details.CompanyIndustry, response.CompanyIndustry)
	require.Equal(t, details.CompanyLocation, response.CompanyLocation)
	require.Equal(t, details.CompanyID, response.CompanyID)
	require.Equal(t, details.EmployerID, response.EmployerID)
	require.Equal(t, details.EmployerFullName, response.EmployerFullName)
	require.Equal(t, details.EmployerEmail, response.EmployerEmail)
}
