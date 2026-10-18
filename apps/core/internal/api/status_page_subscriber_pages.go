package api

import (
	"errors"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/publicpage"
	"orion/core/internal/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	subscribeFlashSent   = "Check your inbox. We sent a link to confirm your email."
	subscribeFlashFailed = "We could not send the confirmation email. Please try again later."
)

// subscriberPageShell is the shell for the token and subscribe pages: never indexed, no canonical address.
func (s *Server) subscriberPageShell(c *gin.Context, page db.StatusPage, title string) (publicpage.Shell, StatusPagePreviewResponse, bool) {
	preview, err := s.publicPreviewFor(page)
	if err != nil {
		utils.InternalError(c, "Failed to load status page", err)
		return publicpage.Shell{}, preview, false
	}
	shell, _ := publicPageShell(c, preview, title+" - "+preview.Page.Title, false)
	shell.Robots = "noindex"
	return shell, preview, true
}

func subscriberOptions(available []StatusPageSubscriberComponentResponse, selected map[string]bool, allWhenEmpty bool) []publicpage.Option {
	options := make([]publicpage.Option, 0, len(available))
	for _, component := range available {
		options = append(options, publicpage.Option{ID: component.ID, Name: component.Name, Checked: selected[component.ID] || (allWhenEmpty && len(selected) == 0)})
	}
	return options
}

func (s *Server) subscribeForm(c *gin.Context, page db.StatusPage, email string, selected map[string]bool, flash string, flashIsError bool) {
	shell, _, ok := s.subscriberPageShell(c, page, "Subscribe to updates")
	if !ok {
		return
	}
	available, err := s.publicSubscriberAvailableComponents(page.ID)
	if err != nil {
		utils.InternalError(c, "Failed to load status page components", err)
		return
	}
	data := publicpage.SubscribeData{Shell: shell, Action: shell.SubscribeURL, Email: email, All: len(selected) == 0, Flash: flash, FlashIsError: flashIsError, Options: subscriberOptions(available, selected, false)}
	data.Hero = publicpage.Hero{Ticker: ticker("Subscribe", publicpage.ToneGood), Eyebrow: "Status updates", Title: "Get status updates", Text: "We email you when something breaks and when it is fixed."}
	status := http.StatusOK
	if flashIsError {
		status = http.StatusBadRequest
	}
	s.writePublicPage(c, status, "subscribe.html", data, true)
}

// getPublicSubscribePage shows the subscribe form.
// @Summary      Public subscribe page
// @Description  HTML form to subscribe to status updates by email
// @Tags         public-status
// @Produce      html
// @ID           getPublicStatusPageSubscribePage
// @Param        slug  path      string  true  "Status page slug"
// @Success      200   {string}  string  "HTML page"
// @Failure      404   {object}  utils.APIResponse
// @Router       /status/{slug}/subscribe [get]
func (s *Server) getPublicSubscribePage(c *gin.Context) {
	page, err := s.loadPublicStatusPageForRequest(c, c.Param("slug"))
	if err != nil {
		writeStatusPageLoadError(c, err, "Status page not found")
		return
	}
	s.subscribeForm(c, page, "", map[string]bool{}, "", false)
}

// postPublicSubscribePage takes the subscribe form. It answers the same whether or not the address was
// already subscribed, so the form cannot be used to find out who is on a list.
// @Summary      Submit public subscribe form
// @Description  Form submit of the subscribe page; mails a confirmation link. Answers the same for known and new addresses.
// @Tags         public-status
// @Accept       x-www-form-urlencoded
// @Produce      html
// @ID           postPublicStatusPageSubscribePage
// @Param        slug          path      string    true   "Status page slug"
// @Param        email         formData  string    true   "Email address"
// @Param        scope         formData  string    false  "all or selected" Enums(all, selected)
// @Param        component_ids formData  []string  false  "Components when scope is selected"
// @Success      200   {string}  string  "HTML page"
// @Failure      400   {string}  string  "HTML page with the error"
// @Failure      404   {object}  utils.APIResponse
// @Failure      429   {object}  utils.APIResponse
// @Router       /status/{slug}/subscribe [post]
func (s *Server) postPublicSubscribePage(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "create") {
		return
	}
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return
	}
	email := strings.TrimSpace(c.PostForm("email"))
	selected := map[string]bool{}
	var ids []string
	if c.PostForm("scope") == "selected" {
		for _, id := range c.PostFormArray("component_ids") {
			selected[id] = true
			ids = append(ids, id)
		}
	}
	kind := statusPageSubscriberDestinationEmail
	result, err := s.subscribeToStatusPage(page, &kind, &email, ids)
	if err != nil {
		var invalid *requestValidationError
		if errors.As(err, &invalid) {
			s.subscribeForm(c, page, email, selected, invalid.message, true)
			return
		}
		s.logger.Error("Failed to create status page subscriber", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to create status page subscription", err)
		return
	}
	if result.confirmationDelivery == "failed" || result.confirmationDelivery == "not_configured" {
		s.subscribeForm(c, page, email, selected, subscribeFlashFailed, true)
		return
	}
	s.subscribeForm(c, page, "", map[string]bool{}, subscribeFlashSent, false)
}

func (s *Server) tokenPageData(c *gin.Context, page db.StatusPage, title string) (publicpage.Shell, bool) {
	shell, _, ok := s.subscriberPageShell(c, page, title)
	return shell, ok
}

// confirmSubscriberHTML confirms the address and lands on the "You are subscribed" page.
func (s *Server) confirmSubscriberHTML(c *gin.Context, page db.StatusPage, token string) {
	shell, ok := s.tokenPageData(c, page, "Subscription confirmed")
	if !ok {
		return
	}
	data := publicpage.ManageData{Shell: shell, Retry: shell.SubscribeURL}
	subscriber, err := s.confirmPendingSubscriber(page.ID, hashStatusPageSubscriberToken(token))
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		data.Expired = true
		data.Hero = publicpage.Hero{Ticker: ticker("Expired", publicpage.ToneWarn), Eyebrow: "Status updates", Title: "This link has expired"}
		s.writePublicPage(c, http.StatusNotFound, "manage.html", data, true)
		return
	case err != nil:
		s.logger.Error("Failed to confirm status page subscriber", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to confirm status page subscription", err)
		return
	}
	data.Hero = publicpage.Hero{Ticker: ticker("Subscribed", publicpage.ToneGood), Eyebrow: "Status updates", Title: "You are subscribed"}
	data.Email = subscriber.MaskedDestination
	data.Scope = "Every update email has a link to change what you receive or to unsubscribe."
	s.writePublicPage(c, http.StatusOK, "manage.html", data, true)
}

func (s *Server) manageSubscriberHTML(c *gin.Context, page db.StatusPage, subscriber db.StatusPageSubscriber, flash string) {
	shell, ok := s.tokenPageData(c, page, "Manage subscription")
	if !ok {
		return
	}
	response, err := s.statusPageSubscriberPublicResponse(page.ID, subscriber)
	if err != nil {
		utils.InternalError(c, "Failed to load status page subscriber preferences", err)
		return
	}
	selected := map[string]bool{}
	for _, id := range response.ComponentIDs {
		selected[id] = true
	}
	scope := "All components"
	if n := len(response.ComponentIDs); n > 0 && n < len(response.AvailableComponents) {
		scope = strconv.Itoa(n) + " of " + strconv.Itoa(len(response.AvailableComponents)) + " components"
	}
	data := publicpage.ManageData{Shell: shell, Email: subscriber.MaskedDestination, Scope: scope, Flash: flash, Retry: shell.SubscribeURL,
		Action: shell.BasePath + "/subscribers/manage/" + url.PathEscape(c.Param("token")), Options: subscriberOptions(response.AvailableComponents, selected, true)}
	data.Hero = publicpage.Hero{Ticker: ticker("Subscribed", publicpage.ToneGood), Eyebrow: "Status updates", Title: "You are subscribed"}
	s.writePublicPage(c, http.StatusOK, "manage.html", data, true)
}

func (s *Server) expiredManageHTML(c *gin.Context, page db.StatusPage) {
	shell, ok := s.tokenPageData(c, page, "Link expired")
	if !ok {
		return
	}
	data := publicpage.ManageData{Shell: shell, Expired: true, Retry: shell.SubscribeURL}
	data.Hero = publicpage.Hero{Ticker: ticker("Expired", publicpage.ToneWarn), Eyebrow: "Status updates", Title: "This link has expired"}
	s.writePublicPage(c, http.StatusNotFound, "manage.html", data, true)
}

// postPublicSubscriberPreferencesForm saves the manage form. Unlike the JSON PUT it keeps the links in the
// subscriber's inbox working, because the page is reloaded from the same address.
// @Summary      Save subscriber preferences (form)
// @Description  Form submit of the manage page; keeps the manage and unsubscribe links valid
// @Tags         public-status
// @Accept       x-www-form-urlencoded
// @Produce      html
// @ID           postPublicStatusPageSubscriberPreferencesForm
// @Param        slug          path      string    true   "Status page slug"
// @Param        token         path      string    true   "Manage token"
// @Param        component_ids formData  []string  false  "Selected components; none or all means all"
// @Success      200   {string}  string  "HTML page"
// @Failure      404   {object}  utils.APIResponse
// @Failure      429   {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/manage/{token} [post]
func (s *Server) postPublicSubscriberPreferencesForm(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "manage") {
		return
	}
	page, subscriber, ok := s.loadPublicSubscriberByManageToken(c)
	if !ok {
		return
	}
	if subscriber.State == statusPageSubscriberStateUnsubscribed || subscriber.State == statusPageSubscriberStateDisabled {
		s.expiredManageHTML(c, page)
		return
	}
	available, err := s.publicSubscriberAvailableComponents(page.ID)
	if err != nil {
		utils.InternalError(c, "Failed to update status page subscriber preferences", err)
		return
	}
	ids, err := s.validatePublicSubscriberComponentIDs(page.ID, c.PostFormArray("component_ids"))
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if len(ids) == 0 {
		s.manageSubscriberHTML(c, page, subscriber, "Choose at least one component, or unsubscribe from the link in your emails.")
		return
	}
	if len(ids) == len(available) {
		ids = []string{}
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error { return replaceStatusPageSubscriberComponents(tx, subscriber.ID, ids) }); err != nil {
		s.logger.Error("Failed to update subscriber preferences", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to update status page subscriber preferences", err)
		return
	}
	s.manageSubscriberHTML(c, page, subscriber, "Preferences saved.")
}

func (s *Server) unsubscribeData(c *gin.Context, page db.StatusPage, state string, email string, manageURL string) {
	shell, ok := s.tokenPageData(c, page, "Unsubscribe")
	if !ok {
		return
	}
	data := publicpage.UnsubscribeData{Shell: shell, State: state, Email: email, ManageURL: manageURL, Action: shell.BasePath + "/subscribers/unsubscribe/" + url.PathEscape(c.Param("token"))}
	data.Hero = publicpage.Hero{Ticker: ticker("Unsubscribe", publicpage.ToneWarn), Eyebrow: "Status updates", Title: "Unsubscribe"}
	status := http.StatusOK
	if state == "invalid" {
		status = http.StatusNotFound
	}
	s.writePublicPage(c, status, "unsubscribe.html", data, true)
}

// getPublicUnsubscribePage asks before leaving: a GET never unsubscribes, so mail scanners cannot do it.
// @Summary      Public unsubscribe page
// @Description  Asks for confirmation before unsubscribing; a GET never changes state
// @Tags         public-status
// @Produce      html
// @ID           getPublicStatusPageUnsubscribePage
// @Param        slug   path      string  true  "Status page slug"
// @Param        token  path      string  true  "Unsubscribe token"
// @Success      200    {string}  string  "HTML page"
// @Failure      404    {string}  string  "HTML page for an invalid token"
// @Failure      429    {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/unsubscribe/{token} [get]
func (s *Server) getPublicUnsubscribePage(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "unsubscribe") {
		return
	}
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return
	}
	var subscriber db.StatusPageSubscriber
	err := s.db.Where("status_page_id = ? AND unsubscribe_token_hash = ?", page.ID, hashStatusPageSubscriberToken(c.Param("token"))).First(&subscriber).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		s.unsubscribeData(c, page, "invalid", "", "")
	case err != nil:
		utils.InternalError(c, "Failed to load status page subscriber", err)
	case subscriber.State == statusPageSubscriberStateUnsubscribed:
		s.unsubscribeData(c, page, "done", subscriber.MaskedDestination, "")
	default:
		s.unsubscribeData(c, page, "confirm", subscriber.MaskedDestination, "")
	}
}

func (s *Server) unsubscribeSubscriberHTML(c *gin.Context, page db.StatusPage) {
	found, err := s.unsubscribeByToken(page.ID, c.Param("token"))
	if err != nil {
		s.logger.Error("Failed to unsubscribe status page subscriber", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to unsubscribe status page subscriber", err)
		return
	}
	if !found {
		s.unsubscribeData(c, page, "invalid", "", "")
		return
	}
	s.unsubscribeData(c, page, "done", "", "")
}
