package api

import (
	"net/http"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func serviceNotImplemented(c *gin.Context) {
	utils.ErrorResponse(c, http.StatusNotImplemented, "Services are not implemented yet", nil)
}

func (s *Server) listServicesImpl(c *gin.Context)           { serviceNotImplemented(c) }
func (s *Server) createServiceImpl(c *gin.Context)          { serviceNotImplemented(c) }
func (s *Server) listServiceSuggestionsImpl(c *gin.Context) { serviceNotImplemented(c) }
func (s *Server) getServiceImpl(c *gin.Context)             { serviceNotImplemented(c) }
func (s *Server) updateServiceImpl(c *gin.Context)          { serviceNotImplemented(c) }
func (s *Server) deleteServiceImpl(c *gin.Context)          { serviceNotImplemented(c) }
func (s *Server) linkServiceItemImpl(c *gin.Context)        { serviceNotImplemented(c) }
func (s *Server) unlinkServiceItemImpl(c *gin.Context)      { serviceNotImplemented(c) }
