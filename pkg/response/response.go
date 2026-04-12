package response

import (
	"net/http"

	"cloud-agent-monitor/pkg/model"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, model.SuccessResponse(data))
}

func SuccessWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, model.SuccessWithMessage(message, data))
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, model.SuccessResponse(data))
}

func CreatedWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, model.SuccessWithMessage(message, data))
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func Paged(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, model.PagedResponse(items, total, page, pageSize))
}

func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, model.ErrorResponse(model.CodeBadRequest, message))
}

func InvalidRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, model.ErrorResponse(model.CodeInvalidRequest, message))
}

func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, model.ErrorResponse(model.CodeUnauthorized, message))
}

func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, model.ErrorResponse(model.CodeForbidden, message))
}

func NotFound(c *gin.Context, code, message string) {
	c.JSON(http.StatusNotFound, model.ErrorResponse(code, message))
}

func NotFoundDefault(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, model.ErrorResponse(model.CodeNotFound, message))
}

func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, model.ErrorResponse(model.CodeConflict, message))
}

func InternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, model.ErrorResponse(model.CodeInternalError, message))
}

func ServiceUnavailable(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, model.ErrorResponse(model.CodeServiceUnavailable, message))
}

func TokenExpired(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, model.ErrorResponse(model.CodeTokenExpired, "token has expired"))
}

func InvalidToken(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, model.ErrorResponse(model.CodeInvalidToken, "invalid token"))
}

func Error(c *gin.Context, err error) {
	apiErr, ok := model.GetAPIError(err)
	if ok {
		c.JSON(apiErr.HTTPStatus(), model.ErrorResponseFromError(err))
		return
	}
	InternalError(c, err.Error())
}

func ErrorWithCode(c *gin.Context, httpStatus int, code, message string) {
	c.JSON(httpStatus, model.ErrorResponse(code, message))
}

func FromAPIError(c *gin.Context, err *model.APIError) {
	c.JSON(err.HTTPStatus(), model.ErrorResponseFromError(err))
}
