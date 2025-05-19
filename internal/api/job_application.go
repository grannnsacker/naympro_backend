package api

import (
	"database/sql"
	"fmt"
	"github.com/gin-gonic/gin"
	db "github.com/grannnsacker/job-finder-back/internal/db/sqlc"
	"github.com/grannnsacker/job-finder-back/pkg/token"
	"github.com/lib/pq"
	"io"
	"net/http"
	"strconv"
	"time"
)

// jobApplicationDoesNotExistError return job application does not exist error
func jobApplicationDoesNotExistError(id int32) error {
	return fmt.Errorf("job application with ID %d does not exist", id)
}

// employerNotPartOfCompanyError return employer is not part of the company error
func employerNotPartOfCompanyError(employerID int32) error {
	return fmt.Errorf("employer with ID %d is not part of the company that created this job", employerID)
}

// userNotOwnerOfApplicationError return user is not the owner of the job application error
func userNotOwnerOfApplicationError(userID int32) error {
	return fmt.Errorf("user with ID %d is not the owner of this job application", userID)
}

type changeNotifyJobApplicationRequest struct {
	ID           int32 `json:"id"`
	Notification bool  `json:"notification"`
}

type jobApplicationResponse struct {
	ID        int32                `json:"id"`
	JobID     int32                `json:"job_id"`
	Message   string               `json:"message"`
	Status    db.ApplicationStatus `json:"status"`
	AppliedAt time.Time            `json:"applied_at"`
}

func newJobApplicationResponse(jobApplication db.JobApplication) jobApplicationResponse {
	return jobApplicationResponse{
		ID:        jobApplication.ID,
		JobID:     jobApplication.JobID,
		Message:   jobApplication.Message.String,
		Status:    jobApplication.Status,
		AppliedAt: jobApplication.AppliedAt,
	}
}

// @Schemes
// @Summary Create job application
// @Description Create a job application. Only users can access this endpoint.
// @Tags job applications
// @param cv formData file true "CV file (.pdf)"
// @param message formData string false "Message for the employer"
// @param job_id formData int true "Job ID"
// @Accept multipart/form-data
// @Produce json
// @Success 200 {object} jobApplicationResponse
// @Failure 400 {object} ErrorResponse "Invalid request body"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only users can access, not employers."
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications [post]
// createJobApplication creates a new job application
func (server *Server) createJobApplication(ctx *gin.Context) {
	// check if the user is authenticated
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authUser, err := server.store.GetUserByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by the employer
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the CV file
	file, header, err := ctx.Request.FormFile("cv")
	if err != nil || header == nil {
		err = fmt.Errorf("valid CV file is required: %w", err)
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	defer file.Close()

	// Read the file data and convert it to a byte slice
	cvData, err := io.ReadAll(file)
	if err != nil {
		err = fmt.Errorf("failed to read the CV file: %w", err)
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the message and the jobID
	message := ctx.Request.FormValue("message")
	jobIDStr := ctx.Request.FormValue("job_id")

	// Validate the jobID
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil || jobID <= 0 {
		err = fmt.Errorf("invalid job ID. Please provide a valid positive integer job ID")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// create job application in the database
	params := db.CreateJobApplicationTxParams{
		CreateJobApplicationParams: db.CreateJobApplicationParams{
			UserID: authUser.ID,
			JobID:  int32(jobID),
			Message: sql.NullString{
				String: message,
				Valid:  len(message) > 0,
			},
			Cv: cvData,
		},
		AfterCreate: func(jobApplication db.JobApplication) error {
			return nil
		},
	}

	txResult, err := server.store.CreateJobApplicationTx(ctx, params)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code.Name() {
			case "unique_violation":
				err := fmt.Errorf("user with ID %d has already applied for this job", authUser.ID)
				ctx.JSON(http.StatusForbidden, errorResponse(err))
				return
			}
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusCreated, newJobApplicationResponse(txResult.JobApplication))
}

type getJobApplicationForUserRequest struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

type getJobApplicationForUserResponse struct {
	ApplicationID      int32                `json:"application_id"`
	JobID              int32                `json:"job_id"`
	JobTitle           string               `json:"job_title"`
	CompanyName        string               `json:"company_name"`
	ApplicationStatus  db.ApplicationStatus `json:"application_status"`
	ApplicationDate    time.Time            `json:"application_date"`
	ApplicationMessage string               `json:"application_message"`
	CvLink             string               `json:"cv_link"`
	UserID             int32                `json:"user_id"`
}

// @Schemes
// @Summary Get job application for user
// @Description Get job application for a user. Only users can access this endpoint. It returns different details than getJobApplicationForEmployer.
// @Tags job applications
// @param id path int true "job application ID"
// @Produce json
// @Success 200 {object} getJobApplicationForUserResponse
// @Failure 400 {object} ErrorResponse "Invalid ID"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only users can access, not employers."
// @Failure 403 {object} ErrorResponse "Only the applicant (the owner) of the job application can access this endpoint."
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/user/{id} [get]
// getJobApplicationForUser gets a job application for a user.
// Only users can access this endpoint and only the applicant (the owner)
// of the job application will receive the success response.
// It also returns different details than getJobApplicationForEmployer
// (suitable for the user needs)
func (server *Server) getJobApplicationForUser(ctx *gin.Context) {
	var request getJobApplicationForUserRequest
	if err := ctx.ShouldBindUri(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the user is authenticated (and is a user, not employer)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authUser, err := server.store.GetUserByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by an employer
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the job application from the database
	jobApplication, err := server.store.GetJobApplicationForUser(ctx, request.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(
				jobApplicationDoesNotExistError(request.ID),
			))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// check if the authenticated user is the applicant
	if authUser.ID != jobApplication.UserID {
		err = fmt.Errorf("user with ID %d is not the applicant of this job application", authUser.ID)
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// serve the CV as a pdf file
	fileName := fmt.Sprintf("cv_%d.pdf", jobApplication.ApplicationID)
	url := fmt.Sprintf("assets/cvs/%s", fileName)
	server.router.GET(url, func(c *gin.Context) {
		// Set the appropriate response headers to indicate it's a PDF
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileName))

		c.Data(http.StatusOK, "application/pdf", jobApplication.UserCv)
	})

	res := getJobApplicationForUserResponse{
		ApplicationID:     jobApplication.ApplicationID,
		JobID:             jobApplication.JobID,
		JobTitle:          jobApplication.JobTitle,
		CompanyName:       jobApplication.CompanyName,
		ApplicationStatus: jobApplication.ApplicationStatus,
		ApplicationDate:   jobApplication.ApplicationDate,
		CvLink:            fmt.Sprintf("%s/%s", server.config.ServerAddress, url),
	}
	if jobApplication.ApplicationMessage.Valid {
		res.ApplicationMessage = jobApplication.ApplicationMessage.String
	}

	ctx.JSON(http.StatusOK, res)
}

type getJobApplicationForEmployerRequest struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

type getJobApplicationForEmployerResponse struct {
	ApplicationID      int32                `json:"application_id"`
	JobTitle           string               `json:"job_title"`
	JobID              int32                `json:"job_id"`
	ApplicationStatus  db.ApplicationStatus `json:"application_status"`
	ApplicationDate    time.Time            `json:"application_date"`
	ApplicationMessage string               `json:"application_message"`
	UserID             int32                `json:"user_id"`
	UserEmail          string               `json:"user_email"`
	UserFullName       string               `json:"user_full_name"`
	UserLocation       string               `json:"user_location"`
	CvLink             string               `json:"cv_link"`
}

// @Schemes
// @Summary Get job application for employer
// @Description Get job application for an employer. Only employers can access this endpoint. It returns different details than getJobApplicationForUser.
// @Tags job applications
// @param id path int true "job application ID"
// @Produce json
// @Success 200 {object} getJobApplicationForEmployerResponse
// @Failure 400 {object} ErrorResponse "Invalid ID"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only employers can access, not users."
// @Failure 403 {object} ErrorResponse "Only an employer that is part of the company that created the job that this application is for can access this endpoint.
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/employer/{id} [get]
// getJobApplicationForEmployer gets a job application for an employer.
// Only employers can access this endpoint and only employers that are part
// of the company will receive the success response.
// It also returns different details than getJobApplicationForUser
// (suitable for the employer needs)
func (server *Server) getJobApplicationForEmployer(ctx *gin.Context) {
	var request getJobApplicationForEmployerRequest
	if err := ctx.ShouldBindUri(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the employer is authenticated (and is an employer, not user)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authEmployer, err := server.store.GetEmployerByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by a user
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyEmployersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the job application from the database
	jobApplication, err := server.store.GetJobApplicationForEmployer(ctx, request.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(
				jobApplicationDoesNotExistError(request.ID),
			))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// check if the authenticated employer is part of the company
	// that created this application
	companyID, err := server.store.GetCompanyIDOfJob(ctx, jobApplication.JobID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	if companyID != authEmployer.CompanyID {
		ctx.JSON(http.StatusForbidden, errorResponse(
			employerNotPartOfCompanyError(authEmployer.ID),
		))
		return
	}

	res := getJobApplicationForEmployerResponse{
		ApplicationID:     jobApplication.ApplicationID,
		JobTitle:          jobApplication.JobTitle,
		JobID:             jobApplication.JobID,
		ApplicationStatus: jobApplication.ApplicationStatus,
		ApplicationDate:   jobApplication.ApplicationDate,
		UserID:            jobApplication.UserID,
		UserEmail:         jobApplication.UserEmail,
		UserFullName:      jobApplication.UserFullName,
		UserLocation:      jobApplication.UserLocation,
	}

	if jobApplication.ApplicationMessage.Valid {
		res.ApplicationMessage = jobApplication.ApplicationMessage.String
	}

	// if the application status was 'Applied', change it to `Seen`
	if jobApplication.ApplicationStatus == db.ApplicationStatusApplied {
		err = server.store.UpdateJobApplicationStatus(ctx, db.UpdateJobApplicationStatusParams{
			ID:     jobApplication.ApplicationID,
			Status: db.ApplicationStatusSeen,
		})
		res.ApplicationStatus = db.ApplicationStatusSeen

		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
	}

	// serve the CV as a pdf file
	fileName := fmt.Sprintf("cv_%d.pdf", jobApplication.ApplicationID)
	url := fmt.Sprintf("assets/cvs/%s", fileName)
	server.router.GET(url, func(c *gin.Context) {
		// Set the appropriate response headers to indicate it's a PDF
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%s", fileName))

		c.Data(http.StatusOK, "application/pdf", jobApplication.UserCv)
	})
	res.CvLink = fmt.Sprintf("%s/%s", server.config.ServerAddress, url)

	ctx.JSON(http.StatusOK, res)
}

type changeJobApplicationStatusUriRequest struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

type changeJobApplicationStatusRequest struct {
	NewStatus db.ApplicationStatus `json:"new_status" binding:"required,oneof=Interviewing Offered Rejected"`
}

type changeJobApplicationStatusResponse struct {
	ApplicationID int32                `json:"application_id"`
	Status        db.ApplicationStatus `json:"status"`
	Message       string               `json:"message"`
}

// @Schemes
// @Summary Change job application status (employer)
// @Description Change job application status as an employer. Only employers can access this endpoint.
// @Tags job applications
// @param id path int true "job application ID"
// @param new_status body changeJobApplicationStatusRequest true "new status"
// @Produce json
// @Success 200 {object} changeJobApplicationStatusResponse
// @Failure 400 {object} ErrorResponse "Invalid status or job application ID"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only employers can access, not users."
// @Failure 403 {object} ErrorResponse "Only an employer that is part of the company that created the job that this application is for can access this endpoint.
// @Failure 404 {object} ErrorResponse "Job application with given ID does not exist"
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/employer/{id}/status [patch]
// changeJobApplicationStatus allows employer to change the status of a job application.
func (server *Server) changeJobApplicationStatus(ctx *gin.Context) {
	var uriRequest changeJobApplicationStatusUriRequest
	if err := ctx.ShouldBindUri(&uriRequest); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var request changeJobApplicationStatusRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the employer is authenticated (and is an employer, not user)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authEmployer, err := server.store.GetEmployerByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by a user
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyEmployersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get job of the job application with given ID
	jobID, err := server.store.GetJobIDOfJobApplication(ctx, uriRequest.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(
				jobApplicationDoesNotExistError(uriRequest.ID),
			))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// check if the employer is part of the company that created job for which the application was made
	// get company ID
	companyID, err := server.store.GetCompanyIDOfJob(ctx, jobID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// compare companyID and employers company id
	if companyID != authEmployer.CompanyID {
		ctx.JSON(http.StatusForbidden, errorResponse(
			employerNotPartOfCompanyError(authEmployer.ID),
		))
		return
	}

	// update the job application status
	err = server.store.UpdateJobApplicationStatus(ctx, db.UpdateJobApplicationStatusParams{
		ID:     uriRequest.ID,
		Status: request.NewStatus,
	})

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	res := changeJobApplicationStatusResponse{
		ApplicationID: uriRequest.ID,
		Status:        request.NewStatus,
		Message:       "Status updated successfully",
	}

	ctx.JSON(http.StatusOK, res)
}

type updateJobApplicationRequest struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

// @Schemes
// @Summary Update job application (user)
// @Description Update job application details (message, cv) but only if the status is 'Applied' (the application was not seen by the employer). Only users can access this endpoint.
// @Tags job applications
// @param id path int true "job application ID"
// @param cv formData file false "CV file (.pdf)"
// @param cv_provided formData boolean true "was CV file provided"
// @param message formData string false "Message for the employer"
// @Produce json
// @Success 200 {object} jobApplicationResponse
// @Failure 400 {object} ErrorResponse "Invalid data or job application ID"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only users can access, not employers."
// @Failure 403 {object} ErrorResponse "Only a user that created this job application can access this endpoint.
// @Failure 404 {object} ErrorResponse "Job application with given ID does not exist"
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/user/{id} [patch]
// updateJobApplication allows users to update a job application.
func (server *Server) updateJobApplication(ctx *gin.Context) {
	var request updateJobApplicationRequest
	if err := ctx.ShouldBindUri(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the user is authenticated (and is a user, not an employer)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authUser, err := server.store.GetUserByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by an employer
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the job application and check if the user created it
	applicationDetails, err := server.store.GetJobApplicationUserIDAndStatus(ctx, request.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(
				jobApplicationDoesNotExistError(request.ID),
			))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// check if the status is 'Applied' - the application was not seen by the employer
	if applicationDetails.Status != db.ApplicationStatusApplied {
		err = fmt.Errorf("job application with ID %d was seen by the employer and cannot be updated anymore", request.ID)
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// compare userID and users ID to check if the user created the job application
	if applicationDetails.UserID != authUser.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(
			userNotOwnerOfApplicationError(authUser.ID),
		))
		return
	}

	// get message and/or cv from the form data

	wasCvProvided := ctx.Request.FormValue("cv_provided")
	var cvData []byte
	if wasCvProvided == "true" || wasCvProvided == "1" {
		// get the CV file
		file, header, err := ctx.Request.FormFile("cv")
		if err != nil {
			// cv_provided was set to true but no actual file was provided
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		defer file.Close()

		// Read the file data and convert it to a byte slice
		if header != nil {
			cvData, err = io.ReadAll(file)
			if err != nil {
				err = fmt.Errorf("failed to read the CV file: %w", err)
				ctx.JSON(http.StatusInternalServerError, errorResponse(err))
				return
			}
		}
	}

	params := db.UpdateJobApplicationParams{
		ID: request.ID,
	}

	// get the message
	message := ctx.Request.FormValue("message")

	// check if the message was provided
	// if it was, set it in the params
	if message != "" {
		params.Message = sql.NullString{
			String: message,
			Valid:  true,
		}
	}

	// check if the CV was provided
	// if it was, set it in the params
	if len(cvData) > 0 {
		params.Cv = cvData
	}

	// update the job application
	jobApplication, err := server.store.UpdateJobApplication(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, newJobApplicationResponse(jobApplication))
}

type deleteJobApplicationRequest struct {
	ID int32 `uri:"id" binding:"required,min=1"`
}

// @Schemes
// @Summary Delete job application (user)
// @Description Delete a job application. Only users can access this endpoint, and only owners of the job application can delete it.
// @Tags job applications
// @param id path int true "job application ID"
// @Success 204 {null} null
// @Failure 400 {object} ErrorResponse "Invalid job application ID"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only users can access, not employers."
// @Failure 403 {object} ErrorResponse "Only a user that created this job application can access this endpoint.
// @Failure 404 {object} ErrorResponse "Job application with given ID does not exist"
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/user/{id} [delete]
// deleteJobApplication delete a job application if the user making
// the request is the owner of the job application
func (server *Server) deleteJobApplication(ctx *gin.Context) {
	var request deleteJobApplicationRequest
	if err := ctx.ShouldBindUri(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the user is authenticated (and is a user, not an employer)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authUser, err := server.store.GetUserByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by an employer
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the userID of the job application and check if the user created it
	userID, err := server.store.GetJobApplicationUserID(ctx, request.ID)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(
				jobApplicationDoesNotExistError(request.ID),
			))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	//  check if the user created the job application
	if userID != authUser.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(
			userNotOwnerOfApplicationError(authUser.ID),
		))
		return
	}

	err = server.store.DeleteJobApplication(ctx, request.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusNoContent, nil)
}

type listJobApplicationsForUser struct {
	Page     int32                `form:"page" binding:"required,min=1"`
	PageSize int32                `form:"page_size" binding:"required,min=5,max=15"`
	Sort     string               `form:"sort" binding:"omitempty,oneof=date-asc date-desc"`
	Status   db.ApplicationStatus `form:"status" binding:"omitempty,oneof=Applied Seen Interviewing Offered Rejected"`
}

// @Schemes
// @Summary List job applications (user)
// @Description List job applications. Only users can access this endpoint. Returns a list of job applications that authenticated user created. Results are paginated based on page and page_size query parameters.
// @Tags job applications
// @param page query int true "page number"
// @param page_size query int true "page size"
// @param sort query string false "sort by date ('date-asc' or 'date-desc')"
// @param status query string false "filter by status ('Applied', 'Seen', 'Interviewing', 'Offered', 'Rejected')"
// @Success 200 {object} []db.ListJobApplicationsForUserRow
// @Failure 400 {object} ErrorResponse "Invalid query parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only users can access, not employers."
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/user [get]
// listJobApplicationsForUser lists all job applications that authenticated
// user created. Results are paginated based on page and page_size query parameters.
func (server *Server) listJobApplicationsForUser(ctx *gin.Context) {
	var request listJobApplicationsForUser
	if err := ctx.ShouldBindQuery(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the user is authenticated (and is a user, not an employer)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authUser, err := server.store.GetUserByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by an employer
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the job applications
	params := db.ListJobApplicationsForUserParams{
		UserID: authUser.ID,
		Limit:  request.PageSize,
		Offset: (request.Page - 1) * request.PageSize,

		// this value does not matter if the FilterStatus is false
		// it just needs to be set to one of the values from the
		// db.ApplicationStatus enum
		Status:       db.ApplicationStatusApplied,
		FilterStatus: false,
	}

	// set ordering of the results
	switch request.Sort {
	case "date-asc":
		params.AppliedAtAsc = true
	default:
		// by default applications will be returned
		// from the newest to the oldest
		params.AppliedAtDesc = true
	}

	// set (if provided) status of the results
	if request.Status != "" {
		params.FilterStatus = true
		params.Status = request.Status
	}

	jobApplications, err := server.store.ListJobApplicationsForUser(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, jobApplications)
}

type listJobApplicationsForEmployer struct {
	JobID    int32                `form:"job_id" binding:"required,min=1"`
	Page     int32                `form:"page" binding:"required,min=1"`
	PageSize int32                `form:"page_size" binding:"required,min=5,max=15"`
	Sort     string               `form:"sort" binding:"omitempty,oneof=date-asc date-desc"`
	Status   db.ApplicationStatus `form:"status" binding:"omitempty,oneof=Applied Seen Interviewing Offered Rejected"`
}

// @Schemes
// @Summary List job applications (employer)
// @Description List job applications for a job with a given ID. Only employers can access this endpoint. Returns a list of job applications that were made for a given job. Results are paginated based on page and page_size query parameters.
// @Tags job applications
// @param job_id query int true "job ID"
// @param page query int true "page number"
// @param page_size query int true "page size"
// @param sort query string false "sort by date ('date-asc' or 'date-desc')"
// @param status query string false "filter by status ('Applied', 'Seen', 'Interviewing', 'Offered', 'Rejected')"
// @Success 200 {object} []db.ListJobApplicationsForEmployerRow
// @Failure 400 {object} ErrorResponse "Invalid query parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized. Only employers can access, not users."
// @Failure 403 {object} ErrorResponse "Employer is trying to access job that does not belong to them."
// @Failure 404 {object} ErrorResponse "Job with given ID does not exist"
// @Failure 500 {object} ErrorResponse "Any other error"
// @Security ApiKeyAuth
// @Router /job-applications/employer [get]
// listJobApplicationsForEmployer lists all job applications for a given job
// that authenticated employer created. Results are paginated based on page and page_size query parameters.
func (server *Server) listJobApplicationsForEmployer(ctx *gin.Context) {
	var request listJobApplicationsForEmployer
	if err := ctx.ShouldBindQuery(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// check if the employer is authenticated (and is an employer, not user)
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	authEmployer, err := server.store.GetEmployerByEmail(ctx, authPayload.Email)
	if err != nil {
		if err == sql.ErrNoRows {
			// but middleware did not stop the request, so it had to be made by a user
			ctx.JSON(http.StatusUnauthorized, errorResponse(onlyUsersAccessError))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// get the job to check if it exists and is owned by the employer
	companyID, err := server.store.GetCompanyIDOfJob(ctx, request.JobID)
	if err != nil {
		if err == sql.ErrNoRows {
			err = fmt.Errorf("job with ID %d does not exist", request.JobID)
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}

		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	// check if the job belongs to the employer
	if companyID != authEmployer.CompanyID {
		err = fmt.Errorf("job with ID %d does not belong to employer with ID %d", request.JobID, authEmployer.CompanyID)
		ctx.JSON(http.StatusForbidden, errorResponse(err))
		return
	}

	// get the job applications
	params := db.ListJobApplicationsForEmployerParams{
		JobID:  request.JobID,
		Limit:  request.PageSize,
		Offset: (request.Page - 1) * request.PageSize,

		// this value does not matter if the FilterStatus is false
		// it just needs to be set to one of the values from the
		// db.ApplicationStatus enum
		Status:       db.ApplicationStatusApplied,
		FilterStatus: false,
	}

	// set ordering of the results
	switch request.Sort {
	case "date-asc":
		params.AppliedAtAsc = true
	default:
		// by default applications will be returned
		// from the newest to the oldest
		params.AppliedAtDesc = true
	}

	// set (if provided) status of the results
	if request.Status != "" {
		params.FilterStatus = true
		params.Status = request.Status
	}

	jobApplications, err := server.store.ListJobApplicationsForEmployer(ctx, params)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, jobApplications)
}

// @Schemes https
// @Summary Change job application notification setting
// @Description Allows authenticated user to enable/disable notifications for their job application. Only the owner of the job application can modify this setting.
// @Tags job applications
// @Param request body changeNotifyJobApplicationRequest true "Notification settings"
// @Success 200 {object} notificationResponse
// @Failure 400 {object} ErrorResponse "Invalid ID format or request body"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "User is trying to modify application that doesn't belong to them"
// @Failure 404 {object} ErrorResponse "Job application not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Security ApiKeyAuth
// @Router /job-applications/user/notifications/ [post]
func (server *Server) changeNotifyJobApplication(ctx *gin.Context) {
	var req changeNotifyJobApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	params := db.UpdateJobApplicationNoteParams{
		ID:           req.ID,
		Notification: req.Notification,
	}

	// Get the job application to check notification status
	err := server.store.UpdateJobApplicationNote(ctx, params)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, notificationResponse{
		Message: "Notification update successfully",
	})
}
