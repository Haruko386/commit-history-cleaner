package common

type ResponseMeta struct {
	RequestID string `json:"requestId"`
}

type SuccessResponse[T any] struct {
	Data T            `json:"data"`
	Meta ResponseMeta `json:"meta"`
}

type Details struct {
	TaskID string `json:"taskId"`
}

type ErrorPayload struct {
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Details   *Details `json:"details,omitempty"`
	Retryable bool     `json:"retryable"`
}

type ErrorResponse struct {
	Error ErrorPayload `json:"error"`
	Meta  ResponseMeta `json:"meta"`
}

func NewSuccessResponse[T any](data T, requestID string) SuccessResponse[T] {
	return SuccessResponse[T]{
		Data: data,
		Meta: ResponseMeta{RequestID: requestID},
	}
}

func NewErrorResponse(code, message string, retryable bool, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorPayload{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
		Meta: ResponseMeta{RequestID: requestID},
	}
}

func NewErrorResponseWithDetails(code, message, details string, retryable bool, requestID string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorPayload{
			Code:      code,
			Message:   message,
			Details:   &Details{TaskID: details},
			Retryable: retryable,
		},
		Meta: ResponseMeta{RequestID: requestID},
	}
}
