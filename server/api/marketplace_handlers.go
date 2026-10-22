package api

import (
	"net/http"

	"c2-dect/server/db"
	"c2-dect/server/marketplace"

	"github.com/gin-gonic/gin"
)

// handleModulesCatalog returns the console catalog. Every module of the Lite
// edition is listed, the ones that are not shipped come back as unavailable.
func (s *Server) handleModulesCatalog(c *gin.Context) {
	modules := s.market.Modules()
	active, inactive := 0, 0
	for _, m := range modules {
		if m.Available {
			active++
		} else {
			inactive++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"count":    len(modules),
		"active":   active,
		"inactive": inactive,
		"modules":  modules,
	})
}

// handleMarketplaceCatalog returns the marketplace packages with the state of
// the installation for the tenant in scope.
func (s *Server) handleMarketplaceCatalog(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	packages := s.market.Packages()
	for i := range packages {
		packages[i].Installed = s.db.IsPackageInstalled(tenantID, packages[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{
		"packages": packages,
		"tenant_id": tenantID,
		"note":      "packages are signed with Ed25519, install only the ones you trust",
	})
}

// handleMarketplacePubkey exposes the signing key of the marketplace.
func (s *Server) handleMarketplacePubkey(c *gin.Context) {
	signer := s.market.Signer()
	if signer == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no signing key configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"kid": signer.Kid(), "pubkey": signer.PubHex()})
}

// handleMarketplaceInstalls lists the packages installed for the tenant.
func (s *Server) handleMarketplaceInstalls(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	installs, err := s.db.ListInstalls(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if installs == nil {
		installs = []db.MarketplaceInstall{}
	}
	c.JSON(http.StatusOK, gin.H{"installs": installs, "tenant_id": tenantID})
}

// handleInstallPackage installs a package for the tenant in scope.
func (s *Server) handleInstallPackage(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	pkg, found := s.market.Get(c.Param("id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}
	if !pkg.Available || !pkg.Installable {
		reason := pkg.Reason
		if reason == "" {
			reason = "the package is not available in this edition"
		}
		s.audit(c, "marketplace", "install_rejected", "warning",
			map[string]any{"package": pkg.ID, "reason": reason})
		c.JSON(http.StatusForbidden, gin.H{"error": reason, "package": pkg.ID, "available": false})
		return
	}
	if err := s.verifyPackage(pkg); err != nil {
		s.audit(c, "marketplace", "install_rejected", "warning",
			map[string]any{"package": pkg.ID, "reason": err.Error()})
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error(), "package": pkg.ID})
		return
	}
	if err := s.db.InstallPackage(tenantID, pkg.ID, pkg.Version, pkg.Checksum, s.callerID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "marketplace", "package_installed", "info",
		map[string]any{"package": pkg.ID, "version": pkg.Version, "checksum": pkg.Checksum})
	c.JSON(http.StatusOK, gin.H{
		"status":   "installed",
		"package":  pkg.ID,
		"version":  pkg.Version,
		"checksum": pkg.Checksum,
	})
}

// handleUninstallPackage removes a package from the tenant in scope.
func (s *Server) handleUninstallPackage(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	pkg, found := s.market.Get(c.Param("id"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}
	if err := s.db.UninstallPackage(tenantID, pkg.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "marketplace", "package_uninstalled", "info", map[string]any{"package": pkg.ID})
	c.JSON(http.StatusOK, gin.H{"status": "uninstalled", "package": pkg.ID})
}

// handleMarketplaceRecipes returns the recipes of the installed packages.
func (s *Server) handleMarketplaceRecipes(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	if !s.db.IsPackageInstalled(tenantID, "lite-recipes") {
		c.JSON(http.StatusOK, gin.H{"recipes": []marketplace.Recipe{}, "locked": true,
			"hint": "install the Lite Recipes package to unlock the recipe catalog"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recipes": s.market.Recipes(), "locked": false})
}

// verifyPackage checks the Ed25519 signature of a package manifest.
func (s *Server) verifyPackage(pkg marketplace.Package) error {
	signer := s.market.Signer()
	if signer == nil {
		return errUnsignedPackage
	}
	if !marketplace.Verify(signer.PubHex(), pkg.Manifest(), pkg.Signature) {
		return errBadSignature
	}
	return nil
}
