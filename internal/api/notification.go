package api

import (
	"database/sql"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
	"net/http"
)

type updateJobApplicationStatusUriRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type updateJobApplicationStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=Interviewing Offered Rejected"`
}

type notificationResponse struct {
	Message string `json:"message"`
}

type NotificationMessage struct {
	ApplicationID int64  `json:"application_id"`
	Status        string `json:"status"`
	UserID        int64  `json:"user_id"`
	JobID         int64  `json:"job_id"`
}

type notifyJobApplicationRequest struct {
	ApplicationID int64  `json:"application_id" binding:"required"`
	Status        string `json:"status" binding:"required,oneof=Interviewing Offered Rejected"`
}

type notificationMessage struct {
	Status      string `json:"status"`
	JobTitle    string `json:"job_title"`
	TelegramID  string `json:"telegram_id"`
	CompanyName string `json:"company_name" binding:"required"`
}

// @Schemes
// @Summary Send job application notification
// @Description Send notification for job application status update
// @Tags job applications
// @Accept json
// @Produce json
// @param NotifyJobApplicationRequest body notifyJobApplicationRequest true "Notification details"
// @Security ApiKeyAuth
// @Success 200 {object} notificationResponse
// @Failure 400 {object} ErrorResponse "Invalid request body"
// @Failure 404 {object} ErrorResponse "Job application not found"
// @Failure 500 {object} ErrorResponse "Any other error"
// @Router /job-applications/notification [post]
func (server *Server) notifyJobApplication(ctx *gin.Context) {
	var req notifyJobApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// Get the job application to check notification status
	application, err := server.store.GetJobApplicationForUser(ctx, int32(req.ApplicationID))
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	// If notification is enabled, send message to RabbitMQ
	if application.Notification {
		// Get user's telegram ID
		user, err := server.store.GetUserByID(ctx, application.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
		// Create notification message
		notification := notificationMessage{
			Status:      req.Status,
			TelegramID:  user.TelegramID,
			CompanyName: application.CompanyName,
			JobTitle:    application.JobTitle,
		}

		// Marshal message to JSON
		body, err := json.Marshal(notification)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}

		// Publish message
		err = server.ch.Publish(
			"",            // exchange
			server.q.Name, // routing key
			false,         // mandatory
			false,         // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
	}

	ctx.JSON(http.StatusOK, notificationResponse{
		Message: "Notification sent successfully",
	})
}
