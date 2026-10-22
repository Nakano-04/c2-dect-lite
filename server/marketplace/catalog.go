// Package marketplace implements the module catalog and the package
// marketplace of C2-DECT Lite.
//
// The Lite edition ships one functional module system: the marketplace. Every
// other module of the console is listed as declared but inactive, so the
// operator sees the full console structure without the edition shipping
// capability it does not implement.
package marketplace

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Availability states of a catalog entry.
const (
	AvailabilityActive   = "active"
	AvailabilityInactive = "inactive"
)

// Module kinds.
const (
	KindCore        = "core"
	KindModule      = "module"
	KindMarketplace = "marketplace"
)

// ModuleDescriptor describes one entry of the console catalog.
type ModuleDescriptor struct {
	Key       string `json:"key"`
	Order     int    `json:"order"`
	Title     string `json:"title"`
	Category  string `json:"category"`
	Kind      string `json:"kind"`
	Summary   string `json:"summary"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Edition   string `json:"edition"`
	Risk      string `json:"risk"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// Package is an installable marketplace package.
type Package struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Author       string   `json:"author"`
	Summary      string   `json:"summary"`
	Capabilities []string `json:"capabilities"`
	Tier         string   `json:"tier"`
	Edition      string   `json:"edition"`
	Size         int      `json:"size"`
	Available    bool     `json:"available"`
	Reason       string   `json:"reason,omitempty"`
	Installable  bool     `json:"installable"`
	Checksum     string   `json:"checksum"`
	Signature    string   `json:"signature,omitempty"`
	Kid          string   `json:"kid,omitempty"`
	Signed       bool     `json:"signed"`
	Installed    bool     `json:"installed"`
}

// RecipeCommand is one step of a recipe.
type RecipeCommand struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Args    string `json:"args"`
}

// Recipe is an ordered list of agent commands that the operator can run with
// a single click. Recipes only use commands from the agent allow list, they
// never carry executable code.
type Recipe struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Package     string          `json:"package"`
	Commands    []RecipeCommand `json:"commands"`
}

// Signer signs package manifests with Ed25519.
type Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	kid  string
}

// NewSigner builds a signer from a 32 byte seed.
func NewSigner(seed []byte) (*Signer, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("marketplace: seed must be %d bytes", ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(pub)
	return &Signer{priv: priv, pub: pub, kid: hex.EncodeToString(sum[:])[:16]}, nil
}

// NewRandomSigner generates a fresh signing key.
func NewRandomSigner() (*Signer, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	return NewSigner(seed)
}

// Kid is the short identifier of the signing key.
func (s *Signer) Kid() string { return s.kid }

// PubHex is the hex encoded public key.
func (s *Signer) PubHex() string { return hex.EncodeToString(s.pub) }

// PublicKey returns the raw public key, para verificar firmas de envelopes.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.pub }

// Sign returns the hex encoded signature of the payload.
func (s *Signer) Sign(payload []byte) string {
	return hex.EncodeToString(ed25519.Sign(s.priv, payload))
}

// Verify checks a signature against a hex encoded public key.
func Verify(pubHex string, payload []byte, sigHex string) bool {
	pub, err := hex.DecodeString(pubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), payload, sig)
}

// Catalog holds the module list and the marketplace packages.
type Catalog struct {
	modules  []ModuleDescriptor
	packages []Package
	recipes  []Recipe
	signer   *Signer
}

// NewCatalog builds the catalog for the Lite edition.
func NewCatalog(signer *Signer) *Catalog {
	c := &Catalog{signer: signer}
	c.modules = buildModules()
	c.packages = buildPackages()
	c.recipes = buildRecipes()
	c.signPackages()
	return c
}

// Modules returns the console catalog.
func (c *Catalog) Modules() []ModuleDescriptor {
	out := make([]ModuleDescriptor, len(c.modules))
	copy(out, c.modules)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// Packages returns the marketplace packages with their signature and checksum.
func (c *Catalog) Packages() []Package {
	out := make([]Package, len(c.packages))
	copy(out, c.packages)
	return out
}

// Get returns one package by identifier.
func (c *Catalog) Get(id string) (Package, bool) {
	for _, p := range c.packages {
		if p.ID == id {
			return p, true
		}
	}
	return Package{}, false
}

// Recipes returns the recipes shipped by the Lite recipes package.
func (c *Catalog) Recipes() []Recipe {
	out := make([]Recipe, len(c.recipes))
	copy(out, c.recipes)
	return out
}

// Signer exposes the signing key metadata.
func (c *Catalog) Signer() *Signer { return c.signer }

// Manifest is the canonical representation that gets signed.
func (p Package) Manifest() []byte {
	payload := struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Version      string   `json:"version"`
		Author       string   `json:"author"`
		Summary      string   `json:"summary"`
		Capabilities []string `json:"capabilities"`
		Tier         string   `json:"tier"`
		Edition      string   `json:"edition"`
	}{
		ID: p.ID, Name: p.Name, Version: p.Version, Author: p.Author,
		Summary: p.Summary, Capabilities: p.Capabilities, Tier: p.Tier, Edition: p.Edition,
	}
	data, _ := json.Marshal(payload)
	return data
}

func (c *Catalog) signPackages() {
	for i := range c.packages {
		p := &c.packages[i]
		m := p.Manifest()
		sum := sha256.Sum256(m)
		p.Checksum = hex.EncodeToString(sum[:])
		p.Size = len(m)
		if c.signer != nil {
			p.Signature = c.signer.Sign(m)
			p.Kid = c.signer.Kid()
			p.Signed = true
		}
	}
}

// inactiveReason es el mensaje de los modulos que la edicion Lite declara
// pero no implementa. Es un esqueleto: el modulo existe como entrada del
// catalogo y nada mas. La razon no describe ninguna capacidad, porque la
// edicion open source no puede publicar la implementacion de un modulo que
// no trae.
const inactiveReason = "Esqueleto: declarado en el catalogo, no implementado en la edicion Lite (open source)."

// buildModules returns the console catalog of the Lite edition.
//
// La lista replica la navegacion de la consola. Los modulos inactivos solo
// tienen nombre y estado: ni codigo, ni configuracion, ni descripcion de lo
// que harian en la edicion comercial.
func buildModules() []ModuleDescriptor {
	return []ModuleDescriptor{
		// --- operate ---
		{Key: "sessions", Order: 10, Title: "Sessions", Category: "operate", Kind: KindCore,
			Summary: "Live agent sessions, tasking and beacon status.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/sessions"},
		{Key: "listeners", Order: 20, Title: "Listeners", Category: "operate", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "payloads", Order: 30, Title: "Payloads", Category: "operate", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "builder", Order: 40, Title: "Payload Builder", Category: "operate", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "loot", Order: 50, Title: "Loot", Category: "operate", Kind: KindCore,
			Summary: "Artifacts collected from the agents of this tenant.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/loot"},
		{Key: "profiles", Order: 60, Title: "Profiles", Category: "operate", Kind: KindCore,
			Summary: "Traffic profiles applied to the agent transport.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/profiles"},
		{Key: "marketplace", Order: 70, Title: "Script Market", Category: "operate", Kind: KindMarketplace,
			Summary: "Marketplace of signed packages and task recipes. The only functional module of the Lite edition.",
			Available: true, Edition: "lite", Risk: "low", Endpoint: "/api/scripts/list"},

		// --- scripts (abierto a la comunidad) ---
		{Key: "lethal-script", Order: 75, Title: "Lethal Script", Category: "operate", Kind: KindCore,
			Summary: "Editor y ejecutor de guiones declarativos: cada paso es una orden del catalogo del agente.",
			Available: true, Edition: "lite", Risk: "medium", Endpoint: "/api/scripts/list"},
		{Key: "lethal-script-docs", Order: 76, Title: "Lethal Script Docs", Category: "research", Kind: KindCore,
			Summary: "Formato de los guiones, limites del sandbox y tecnicas que cubren las ordenes del catalogo.",
			Available: true, Edition: "lite", Risk: "low", Endpoint: "/api/scripts/spec"},

		// --- execution ---
		{Key: "implant-config", Order: 80, Title: "Implant Config", Category: "execution", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},

		// --- elevate / impact ---
		{Key: "admin", Order: 90, Title: "Admin", Category: "impact", Kind: KindCore,
			Summary: "Tenants, operators and agent bootstrap tokens.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/tenants"},

		// --- research ---
		{Key: "dashboard", Order: 100, Title: "Vulnerabilities", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "vuln-scan", Order: 110, Title: "Vuln Scan (PC)", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "hunting-nvd", Order: 120, Title: "Hunting NVD", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "recon-go", Order: 130, Title: "Recon (Go)", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "segment-map", Order: 140, Title: "Segment Map", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "ai-copilot", Order: 150, Title: "IA Copilot", Category: "research", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},

		// --- tools ---
		{Key: "remote-desktop", Order: 160, Title: "Remote Desktop", Category: "tools", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "file-manager", Order: 170, Title: "File Manager", Category: "tools", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "executive", Order: 180, Title: "Executive Summary", Category: "tools", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "recon-vulns", Order: 190, Title: "Recon + Vulns", Category: "tools", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "operations", Order: 200, Title: "Operations", Category: "tools", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},

		// --- tooling ---
		{Key: "tooling", Order: 210, Title: "Recon & Fuzzing", Category: "tooling", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},
		{Key: "bounty", Order: 220, Title: "Bounty & Research", Category: "tooling", Kind: KindModule,
			Available: false, Reason: inactiveReason, Edition: "pro"},

		// --- governance ---
		{Key: "audit", Order: 230, Title: "Audit", Category: "governance", Kind: KindCore,
			Summary: "Tamper evident audit trail of the operator actions.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/audit"},
		{Key: "tenants", Order: 240, Title: "Tenants", Category: "governance", Kind: KindCore,
			Summary: "Tenant and operator administration.", Available: true,
			Edition: "lite", Risk: "low", Endpoint: "/api/tenants"},
	}
}

// buildPackages returns the marketplace catalog of the Lite edition.
func buildPackages() []Package {
	return []Package{
		{
			ID: "lite-core", Name: "Lite Core", Version: "2.0.0", Author: "C2-DECT Contributors",
			Summary: "Core command set of the Lite agent: system inventory, process control, file transfer and tasking.",
			Capabilities: []string{
				"system inventory", "process listing", "file transfer", "tasking", "sleep control",
			},
			Tier: "community", Edition: "lite", Available: true, Installable: true,
		},
		{
			ID: "lite-recipes", Name: "Lite Recipes", Version: "2.0.0", Author: "C2-DECT Contributors",
			Summary: "Predefined task recipes that chain the allowed agent commands into one click routines.",
			Capabilities: []string{"inventory sweep", "file inventory", "connection review"},
			Tier:       "community", Edition: "lite", Available: true, Installable: true,
		},
		{
			ID: "lite-profiles", Name: "Lite Traffic Profiles", Version: "2.0.0", Author: "C2-DECT Contributors",
			Summary: "Traffic profile pack applied to the agent transport (headers, user agent, request path).",
			Capabilities: []string{"traffic profiles", "sleep tuning", "jitter tuning"},
			Tier:       "community", Edition: "lite", Available: true, Installable: true,
		},
		{
			ID: "pro-analytics", Name: "Analytics Pack", Version: "1.0.0", Author: "C2-DECT",
			Summary: "Commercial module.", Tier: "commercial", Edition: "pro",
			Available: false, Installable: false,
			Reason:    "Módulo comercial no incluido en la edición Lite (open source).",
		},
		{
			ID: "pro-orchestration", Name: "Orchestration Pack", Version: "1.0.0", Author: "C2-DECT",
			Summary: "Commercial module.", Tier: "commercial", Edition: "pro",
			Available: false, Installable: false,
			Reason:    "Módulo comercial no incluido en la edición Lite (open source).",
		},
	}
}

// buildRecipes returns the task recipes shipped with the Lite edition.
func buildRecipes() []Recipe {
	return []Recipe{
		{
			ID: "inventory", Name: "Inventory sweep", Package: "lite-recipes",
			Description: "Collects the host inventory, the process list and the network configuration.",
			Commands: []RecipeCommand{
				{Label: "System information", Command: "sysinfo"},
				{Label: "Process list", Command: "ps"},
				{Label: "Network configuration", Command: "netinfo"},
			},
		},
		{
			ID: "connections", Name: "Connection review", Package: "lite-recipes",
			Description: "Lists the running services and the established network connections.",
			Commands: []RecipeCommand{
				{Label: "Running services", Command: "services"},
				{Label: "Network connections", Command: "connections"},
			},
		},
		{
			ID: "files", Name: "File inventory", Package: "lite-recipes",
			Description: "Lists the working directory and searches the temporary folder for documents.",
			Commands: []RecipeCommand{
				{Label: "Working directory", Command: "ls", Args: "."},
				{Label: "Temporary documents", Command: "find", Args: ". *.txt"},
			},
		},
	}
}
