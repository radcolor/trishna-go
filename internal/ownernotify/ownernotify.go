package ownernotify

const maxDiscordMessage = 2000

type Category string

const (
	CategoryNeedsOwner Category = "needs_owner"
	CategoryAskContact Category = "ask_contact"
	CategoryEmotional  Category = "emotional"
	CategorySecurity   Category = "security"
	CategoryImportant  Category = "important"
)

type Result struct {
	Notify   bool
	Category Category
	Summary  string
}
