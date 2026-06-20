package constants

// HTTP header names.
const (
	HeaderAuthorization  = "Authorization"
	HeaderContentType    = "Content-Type"
	HeaderXRequestID     = "X-Request-Id"
	HeaderXNextCursor    = "X-Next-Cursor"
	HeaderXForwardedFor  = "X-Forwarded-For"
	HeaderXRequestedWith = "X-Requested-With"

	BearerPrefix = "Bearer "
)

// MIME / Content-Type values.
const (
	MIMEApplicationJSON           = "application/json"
	MIMETextEventStream           = "text/event-stream"
	MIMEImageGIF                  = "image/gif"
	MIMETextHTMLCharsetUTF8       = "text/html; charset=utf-8"
	MIMEApplicationFormURLEncoded = "application/x-www-form-urlencoded"
	MIMEMultipartFormData         = "multipart/form-data"
	MIMEApplicationOpenAPIJSON    = "application/openapi+json"
)
