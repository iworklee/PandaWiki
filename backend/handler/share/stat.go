package share

import (
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mileusna/useragent"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/handler"
	"github.com/chaitin/panda-wiki/usecase"
)

type ShareStatHandler struct {
	*handler.BaseHandler
	useCase *usecase.StatUseCase
}

func NewShareStatHandler(baseHandler *handler.BaseHandler, echo *echo.Echo, useCase *usecase.StatUseCase) *ShareStatHandler {
	h := &ShareStatHandler{
		BaseHandler: baseHandler,
		useCase:     useCase,
	}

	group := echo.Group("/share/v1/stat")
	group.POST("/page", h.RecordPage)
	return h
}

// RecordPage record page
//
//	@Summary		RecordPage
//	@Description	RecordPage
//	@Tags			share_stat
//	@Accept			json
//	@Produce		json
//	@Param			request	body		domain.StatPageReq	true	"request"
//	@Success		200		{object}	domain.Response
//	@Router			/share/v1/stat/page [post]
func (h *ShareStatHandler) RecordPage(c echo.Context) error {
	req := &domain.StatPageReq{}
	if err := c.Bind(req); err != nil {
		return h.NewResponseWithError(c, "bind request body failed", err)
	}
	if err := c.Validate(req); err != nil {
		return h.NewResponseWithError(c, "validate request body failed", err)
	}

	kbID := c.Request().Header.Get("X-KB-ID")
	userID := ""
	ua := c.Request().UserAgent()
	userAgent := useragent.Parse(ua)
	browserName := userAgent.Name
	browserOS := userAgent.OS
	referer := c.Request().Referer()
	refererHost := ""
	if referer != "" {
		refererURL, err := url.Parse(referer)
		if err == nil {
			refererHost = refererURL.Host
		}
	}
	sessionIDCookie, err := c.Request().Cookie("x-pw-session-id")
	if err != nil {
		return h.NewResponseWithError(c, "get session id failed", err)
	}
	sessionID := sessionIDCookie.Value
	ip := c.RealIP()
	stat := &domain.StatPage{
		KBID:        kbID,
		UserID:      userID,
		NodeID:      req.NodeID,
		Scene:       req.Scene,
		SessionID:   sessionID,
		IP:          ip,
		UA:          ua,
		BrowserName: browserName,
		BrowserOS:   browserOS,
		Referer:     referer,
		RefererHost: refererHost,
		CreatedAt:   time.Now(),
	}
	if err := h.useCase.RecordPage(c.Request().Context(), stat); err != nil {
		return h.NewResponseWithError(c, "record page failed", err)
	}
	return h.NewResponseWithData(c, nil)
}
