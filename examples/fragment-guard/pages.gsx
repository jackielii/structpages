package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
)

// Every GET that shows something is a page (has Page) or a fragment (no
// Page; Props returns RenderComponent). NavigationGuard lands a browser that
// opens a fragment URL on the page that shows it, in one of three ways:
//
//	taskList, renameDialog  default: the nearest ancestor page (project)
//	comments                Landing: built from the query, no lookup
//	mention                 SelfLanding: needs a lookup, so Props lands it
type pages struct {
	home     homePage     `route:"GET /{$} Home"`
	projects projectPages `route:"/projects/{project}"`
	comments comments     `route:"GET /comments Comments"`
	mention  mention      `route:"GET /mentions/{id} Mention"`
}

// A group: the project page is its /{$} index, and its fragments sit beside
// it, so they land on it with no declaration.
type projectPages struct {
	project    projectPage  `route:"GET /{$} Project"`
	tasks      taskList     `route:"GET /tasks Tasks"`
	rename     renameDialog `route:"GET /tasks/{task}/rename Rename task"`
	saveRename saveRename   `route:"POST /tasks/{task}/rename Rename task"`
}

// ---------------------------------------------------------------------------
// Pages

type homePage struct{}

func (homePage) Props(store *Store) ([]Mention, error) {
	return store.Mentions(), nil
}

component (p homePage) Page(mentions []Mention) {
	<Layout title="Home">
		<h1>Fragment guard</h1>
		<p>
			Each link below opens a <em>fragment</em> URL as a page: in a new tab (a native navigation, answered with 303) or
			as a boosted link (an htmx body swap, answered with HX-Location). Either way you land on the page that shows the
			fragment, never on bare markup.
		</p>
		<ul>
			<li>
				<FragmentLink href={taskList{} |> url("project", "42")}>
					Task list
				</FragmentLink>
				: lands on its ancestor, the project page.
			</li>
			<li>
				<FragmentLink href={renameDialog{} |> url("project", "42", "task", "7")}>
					Rename dialog
				</FragmentLink>
				: lands on the project page too.
			</li>
			<li>
				<FragmentLink
					href={
						[]any{comments{}, "?project={project}&task={task}"} |> url(map[string]any{"project": "42", "task": "7"})
					}
				>
					Comments
				</FragmentLink>
				: its Landing opens the project page on that task's comments.
			</li>
			<li>
				<FragmentLink href={mention{} |> url("id", "1")}>
					Mention
				</FragmentLink>
				: SelfLanding. Props looks it up and redirects to its comment.
			</li>
			<li>
				<FragmentLink href={mention{} |> url("id", "3")}>
					Someone else's mention
				</FragmentLink>
				: SelfLanding with an access check, so a 404 page.
			</li>
		</ul>
		<h2>Your mentions</h2>
		<p>Loaded as partials, so these render in place:</p>
		<ul>
			{ for _, m := range mentions {
				<li>
					<button
						hx-get={mention{} |> url("id", m.ID)}
						hx-target="#mention-preview"
					>
						Preview mention { m.ID } (@{ m.Who })
					</button>
				</li>
			} }
		</ul>
		<div id="mention-preview"></div>
	</Layout>
}

component FragmentLink(href string, children gsx.Node) {
	<a href={href}>{ children }</a>
	<small>
		(
		<a href={href} target="_blank" hx-boost="false">new tab</a>)
	</small>
}

// openTask is the ?task= the project page opens comments for.
type openTask string

type projectPage struct{}

func (projectPage) Props(r *http.Request, store *Store) (Project, openTask, error) {
	proj, err := store.Project(r.PathValue("project"))
	return proj, openTask(r.URL.Query().Get("task")), err
}

component (p projectPage) Page(proj Project, open openTask) {
	<Layout title={proj.Name}>
		<h1>{ proj.Name }</h1>
		<div
			id={taskList.List |> id}
			hx-get={taskList{} |> url("project", proj.ID)}
			hx-trigger="load"
		>
			Loading tasks…
		</div>
		{ if open != "" {
			<section
				id="comments"
				hx-get={
					[]any{comments{}, "?project={project}&task={task}"} |> url(map[string]any{"project": proj.ID, "task": string(open)})
				}
				hx-trigger="load"
			>
				Loading comments…
			</section>
		} else {
			<section id="comments"></section>
		} }
		<div id="dialog"></div>
	</Layout>
}

// ---------------------------------------------------------------------------
// Default landing: the ancestor page

type taskList struct{}

func (p taskList) Props(r *http.Request, store *Store) error {
	// Only a partial reaches this: the guard lands page loads.
	projectID := r.PathValue("project")
	if _, err := store.Project(projectID); err != nil {
		return err
	}
	return structpages.RenderComponent(p.List(projectID, store.Tasks(projectID)))
}

component (p taskList) List(projectID string, tasks []Task) {
	<div id={taskList.List |> id}>
		<p.Items projectID={projectID} tasks={tasks}/>
	</div>
}

// Refreshed replaces the list from a response aimed elsewhere (the dialog).
component (p taskList) Refreshed(projectID string, tasks []Task) {
	<div id={taskList.List |> id} hx-swap-oob="true">
		<p.Items projectID={projectID} tasks={tasks}/>
	</div>
}

component (p taskList) Items(projectID string, tasks []Task) {
	<ul>
		{ for _, t := range tasks {
			<li>
				{ t.Title }
				<button
					hx-get={
						[]any{comments{}, "?project={project}&task={task}"} |> url(map[string]any{"project": projectID, "task": t.ID})
					}
					hx-target="#comments"
				>
					Comments
				</button>
				<button
					hx-get={renameDialog{} |> url("project", projectID, "task", t.ID)}
					hx-target="#dialog"
				>
					Rename
				</button>
			</li>
		} }
	</ul>
}

// ---------------------------------------------------------------------------
// One route type per method: the dialog (GET) and the save (POST) share
// loadTask, and neither calls the other.

func loadTask(r *http.Request, store *Store) (Task, error) {
	return store.Task(r.PathValue("project"), r.PathValue("task"))
}

type renameDialog struct{}

// Props shows the dialog: no form parsing, no writes.
func (p renameDialog) Props(r *http.Request, store *Store) error {
	task, err := loadTask(r, store)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(p.Dialog(task, task.Title, ""))
}

component (p renameDialog) Dialog(task Task, title string, problem string) {
	<dialog open>
		<form
			hx-post={saveRename{} |> url("project", task.ProjectID, "task", task.ID)}
			hx-target="#dialog"
		>
			<label>
				Title
				<input name="title" value={title}/>
			</label>
			{ if problem != "" {
				<p role="alert">{ problem }</p>
			} }
			<button>Save</button>
		</form>
	</dialog>
}

type saveRename struct{}

// ServeHTTP saves the new title: the only place that parses the form and
// writes. It answers with an empty dialog and the refreshed task list.
func (saveRename) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
	task, err := loadTask(r, store)
	if err != nil {
		return err
	}
	title := strings.TrimSpace(r.PostFormValue("title"))
	if title == "" {
		return structpages.RenderComponent(renameDialog{}.Dialog(task, title, "A task needs a title."))
	}
	if err := store.RenameTask(task.ProjectID, task.ID, title); err != nil {
		return err
	}
	return structpages.RenderComponent(taskList{}.Refreshed(task.ProjectID, store.Tasks(task.ProjectID)))
}

// ---------------------------------------------------------------------------
// Landing: the right page isn't an ancestor, but the request names it

type comments struct{}

// Landing opens the project page on the task's comments. It reads only the
// query: no lookup, so it can't reveal anything the URL didn't say.
func (comments) Landing(r *http.Request) (string, error) {
	q := r.URL.Query()
	return structpages.URLFor(r.Context(), []any{projectPage{}, "?task={task}#comments"},
		map[string]any{"project": q.Get("project"), "task": q.Get("task")})
}

func (p comments) Props(r *http.Request, store *Store) error {
	q := r.URL.Query()
	task, err := store.Task(q.Get("project"), q.Get("task"))
	if err != nil {
		return err
	}
	return structpages.RenderComponent(p.Thread(task, store.Comments(task.ID)))
}

component (p comments) Thread(task Task, list []Comment) {
	<h2>Comments on “{ task.Title }”</h2>
	{ for _, c := range list {
		<blockquote id={"comment-" + c.ID}>
			<strong>{ c.Author }</strong>: { c.Text }
		</blockquote>
	} }
}

// ---------------------------------------------------------------------------
// SelfLanding: the page depends on the record

type mention struct{}

// LandsItself tells the guard to let page loads through: only a lookup
// knows which task this mention is on, and whether the reader may see it.
func (mention) LandsItself() {}

func (p mention) Props(w http.ResponseWriter, r *http.Request, store *Store) error {
	m, err := store.Mention(r.PathValue("id")) // the access check: not theirs → not found
	if err != nil {
		return err
	}
	if !Partial(r) {
		to, err := structpages.URLFor(r.Context(), []any{projectPage{}, "?task={task}#comments"},
			map[string]any{"project": m.ProjectID, "task": m.TaskID})
		if err != nil {
			return err
		}
		http.Redirect(w, r, to, http.StatusSeeOther)
		return structpages.ErrSkipPageRender
	}
	c, err := store.Comment(m.CommentID)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(p.Card(m, c))
}

component (p mention) Card(m Mention, c Comment) {
	<blockquote>
		<strong>{ c.Author }</strong>
		: { c.Text }
		<footer>
			<a
				href={
					[]any{projectPage{}, "?task={task}#comments"} |> url(map[string]any{"project": m.ProjectID, "task": m.TaskID})
				}
			>
				Open the task
			</a>
		</footer>
	</blockquote>
}

// ---------------------------------------------------------------------------
// Layout and errors

component Layout(title string, children gsx.Node) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<link rel="stylesheet" href="https://unpkg.com/missing.css@1.1.3"/>
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
			<title>{ title }</title>
		</head>
		<body hx-boost="true">
			<header class="navbar">
				<nav>
					<a href={homePage{} |> url}>Home</a>
					<a href={projectPage{} |> url("project", "42")}>Project</a>
				</nav>
			</header>
			<main id="content">{ children }</main>
		</body>
	</html>
}

component ErrorPage(status int) {
	<Layout title={http.StatusText(status)}>
		<ErrorMessage status={status}/>
		<p>
			<a href={homePage{} |> url}>Back to the start</a>
		</p>
	</Layout>
}

component ErrorMessage(status int) {
	<p role="alert">{ strconv.Itoa(status) } { http.StatusText(status) }</p>
}
