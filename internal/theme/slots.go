package theme

// Slot names one style a theme defines, as "section.slot". Every slot is
// listed in themes/default.toml; components draw only through these.
type Slot string

// App chrome.
const (
	UIBase            Slot = "ui.base"
	UIPaneBorder      Slot = "ui.pane_border"
	UIPaneBorderFocus Slot = "ui.pane_border_focus"
	UIStatusline      Slot = "ui.statusline"
	UIStatusFile      Slot = "ui.status_file"
	UIStatusDirty     Slot = "ui.status_dirty"
	UIModeNormal      Slot = "ui.mode_normal"
	UIModeInsert      Slot = "ui.mode_insert"
	UIModeVisual      Slot = "ui.mode_visual"
	UIModeCommand     Slot = "ui.mode_command"
	UIMessage         Slot = "ui.message"
	UIError           Slot = "ui.error"
	UIEndOfBuffer     Slot = "ui.end_of_buffer"
)

// File-tree sidebar.
const (
	SidebarHeader       Slot = "sidebar.header"
	SidebarDir          Slot = "sidebar.dir"
	SidebarFile         Slot = "sidebar.file"
	SidebarOtherFile    Slot = "sidebar.other_file"
	SidebarHidden       Slot = "sidebar.hidden"
	SidebarSelected     Slot = "sidebar.selected"
	SidebarSelectedBlur Slot = "sidebar.selected_blur"
	SidebarOpenFile     Slot = "sidebar.open_file"
)

// Overlays (palettes such as the Task List and New Zettel prompt).
const (
	OverlayBackdrop    Slot = "overlay.backdrop"
	OverlayBox         Slot = "overlay.box"
	OverlayBorder      Slot = "overlay.border"
	OverlayTitle       Slot = "overlay.title"
	OverlaySelected    Slot = "overlay.selected"
	OverlayInput       Slot = "overlay.input"
	OverlayPlaceholder Slot = "overlay.placeholder"
	OverlayHint        Slot = "overlay.hint"
	OverlayGroup       Slot = "overlay.group"
)

// Editor text.
const (
	MarkdownHeading  Slot = "markdown.heading"
	MarkdownMarker   Slot = "markdown.marker"
	MarkdownBold     Slot = "markdown.bold"
	MarkdownItalic   Slot = "markdown.italic"
	MarkdownStrike   Slot = "markdown.strike"
	MarkdownCode     Slot = "markdown.code"
	MarkdownLink     Slot = "markdown.link"
	MarkdownBullet   Slot = "markdown.bullet"
	MarkdownQuote    Slot = "markdown.quote"
	MarkdownTaskBox  Slot = "markdown.task_box"
	MarkdownTaskDone Slot = "markdown.task_done"
	MarkdownTag      Slot = "markdown.tag"
	MarkdownMeta     Slot = "markdown.meta"
	MarkdownSearch   Slot = "markdown.search"
	MarkdownVisual   Slot = "markdown.visual"
)

// Syntax colours inside fenced code blocks.
const (
	CodeKeyword  Slot = "code.keyword"
	CodeString   Slot = "code.string"
	CodeComment  Slot = "code.comment"
	CodeNumber   Slot = "code.number"
	CodeType     Slot = "code.type"
	CodeFunction Slot = "code.function"
	CodeOperator Slot = "code.operator"
)

// Task List.
const (
	TasksOverdue  Slot = "tasks.overdue"
	TasksToday    Slot = "tasks.today"
	TasksUpcoming Slot = "tasks.upcoming"
	TasksNoDate   Slot = "tasks.no_date"
	TasksPriHigh  Slot = "tasks.pri_high"
	TasksSource   Slot = "tasks.source"
)
