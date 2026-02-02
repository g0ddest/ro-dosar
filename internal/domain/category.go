package domain

// Category represents the article category for citizenship application
type Category string

const (
	CategoryArt8   Category = "ART_8"
	CategoryArt8_1 Category = "ART_8_1"
	CategoryArt8_2 Category = "ART_8_2"
	CategoryArt10  Category = "ART_10"
	CategoryArt11  Category = "ART_11"
)

// CategoryInfo contains metadata about a category
type CategoryInfo struct {
	Code        string
	Name        string
	NameRO      string
	Description string
}

// categoryInfoMap contains metadata for all categories
// Text from Legea cetățeniei române nr. 21/1991, republicată
var categoryInfoMap = map[Category]CategoryInfo{
	CategoryArt8: {
		Code:   "ART_8",
		Name:   "Article 8",
		NameRO: "Articolul 8",
		Description: `(1) Cetățenia română se poate acorda, la cerere, persoanei fără cetățenie sau cetățeanului străin, dacă îndeplinește următoarele condiții:

a) s-a născut și domiciliază la data cererii pe teritoriul României sau, deși nu s-a născut pe acest teritoriu, domiciliază pe teritoriul statului român de cel puțin 8 ani sau de cel puțin 5 ani, în cazul în care este căsătorit și conviețuiește cu un cetățean român;

b) dovedește, prin comportament, acțiuni și atitudine, loialitate față de statul român, nu întreprinde sau sprijină acțiuni împotriva ordinii de drept sau a securității naționale și declară că nici în trecut nu a întreprins asemenea acțiuni;

c) a împlinit vârsta de 18 ani;

d) are asigurate în România mijloace legale pentru o existență decentă, în condițiile stabilite de legislația privind regimul străinilor;

e) este cunoscut cu o bună comportare și nu a fost condamnat în țară sau în străinătate pentru o infracțiune care îl face nedemn de a fi cetățean român;

f) cunoaște limba română și posedă noțiuni elementare de cultură și civilizație românească, în măsură suficientă pentru a se integra în viața socială;

g) cunoaște prevederile Constituției României și imnul național.`,
	},
	CategoryArt8_1: {
		Code:        "ART_8_1",
		Name:        "Article 8¹",
		NameRO:      "Articolul 8¹",
		Description: `Cetățenia română se poate acorda și persoanei care a avut această cetățenie și care cere redobândirea ei, cu păstrarea cetățeniei străine și stabilirea domiciliului în țară sau cu menținerea acestuia în străinătate, dacă îndeplinește condițiile prevăzute la art. 8 alin. (1) lit. b), c), e) și f).`,
	},
	CategoryArt8_2: {
		Code:        "ART_8_2",
		Name:        "Article 8²",
		NameRO:      "Articolul 8²",
		Description: `Persoanele care au dobândit cetățenia română prin naștere sau prin adopție și care au pierdut-o din motive neimputabile lor sau această cetățenie le-a fost ridicată fără voia lor, precum și descendenții acestora până la gradul III, la cerere, pot redobândi sau li se poate acorda cetățenia română, cu posibilitatea păstrării cetățeniei străine și stabilirea domiciliului în țară sau cu menținerea acestuia în străinătate, dacă îndeplinesc condițiile prevăzute la art. 8 alin. (1) lit. b), c) și e).`,
	},
	CategoryArt10: {
		Code:   "ART_10",
		Name:   "Article 10",
		NameRO: "Articolul 10",
		Description: `(1) Copilul născut din părinți cetățeni străini sau fără cetățenie și care nu a împlinit vârsta de 18 ani dobândește cetățenia română odată cu părinții săi.

(2) În cazul în care numai unul dintre părinți dobândește cetățenia română, cetățenia copilului se va stabili prin acordul părinților, iar în lipsa acordului va decide instanța de tutelă, ținând seama de interesul superior al copilului.

(3) Dacă copilul a împlinit vârsta de 14 ani, se cere consimțământul acestuia.`,
	},
	CategoryArt11: {
		Code:   "ART_11",
		Name:   "Article 11",
		NameRO: "Articolul 11",
		Description: `(1) Persoanele care au dobândit cetățenia română prin naștere sau prin adopție și care au pierdut-o din motive neimputabile lor sau această cetățenie le-a fost ridicată fără voia lor, precum și descendenții acestora până la gradul III, la cerere, pot redobândi sau li se poate acorda cetățenia română, cu posibilitatea păstrării cetățeniei străine și stabilirea domiciliului în țară sau cu menținerea acestuia în străinătate, dacă îndeplinesc condițiile prevăzute la art. 8 alin. (1) lit. b), c) și e).

(2) Dispozițiile art. 10 alin. (2) și (3) se aplică în mod corespunzător.`,
	},
}

// IsValid checks if the category is a valid value
func (c Category) IsValid() bool {
	_, ok := categoryInfoMap[c]
	return ok
}

// String returns the string representation of the category
func (c Category) String() string {
	return string(c)
}

// Name returns the English name of the category
func (c Category) Name() string {
	if info, ok := categoryInfoMap[c]; ok {
		return info.Name
	}
	return string(c)
}

// NameRO returns the Romanian name of the category
func (c Category) NameRO() string {
	if info, ok := categoryInfoMap[c]; ok {
		return info.NameRO
	}
	return string(c)
}

// Description returns the legal description of the category
func (c Category) Description() string {
	if info, ok := categoryInfoMap[c]; ok {
		return info.Description
	}
	return ""
}

// Info returns the full category information
func (c Category) Info() CategoryInfo {
	if info, ok := categoryInfoMap[c]; ok {
		return info
	}
	return CategoryInfo{Code: string(c), Name: string(c), NameRO: string(c)}
}
