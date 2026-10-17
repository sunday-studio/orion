package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

type SearchHitResponse struct {
	Type   string `json:"type" enums:"monitor,incident,server,app"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

// searchEverything powers the command palette.
// @Summary      Search monitors, incidents, servers and apps
// @Description  Case-insensitive name search, at most 5 hits per type, grouped by type in this order: monitor, incident, server, app.
// @Tags         search
// @Produce      json
// @ID           searchEverything
// @Param        q  query  string  true  "Search text"
// @Success      200  {object}  utils.APIResponse{data=object{hits=[]SearchHitResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/search [get]
func (s *Server) searchEverything(c *gin.Context) {
	hits, err := service.Search(s.db, c.Query("q"))
	if err != nil {
		utils.InternalError(c, "Search failed", err)
		return
	}
	out := make([]SearchHitResponse, 0, len(hits))
	for _, h := range hits {
		out = append(out, SearchHitResponse{Type: h.Type, ID: h.ID, Name: h.Name, Status: h.Status})
	}
	utils.SuccessResponse(c, http.StatusOK, "Search results", gin.H{"hits": out, "count": len(out)})
}
