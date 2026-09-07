package httpapi

import "github.com/gin-gonic/gin"

func (s *Server) clientCreateHost(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.createHost(c)
}

func (s *Server) clientUpdateHost(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.updateHost(c)
}

func (s *Server) clientDeleteHost(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.deleteHost(c)
}

func (s *Server) clientImportHosts(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.importHosts(c)
}

func (s *Server) clientCreateGroup(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.createGroup(c)
}

func (s *Server) clientDeleteGroup(c *gin.Context) {
	if _, ok := s.requireClientCatalogWrite(c); !ok {
		return
	}
	s.deleteGroup(c)
}
