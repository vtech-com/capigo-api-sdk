package cmd

import "net/url"

// A task has two addresses: its id, a UUID, and its code — the human key a
// person quotes, like ACMEC-68. The API exposes both, and the CLI takes the
// same view: one command per capability, addressed either way.
//
// A bare argument is never sniffed. Deciding "this looks like a UUID, so it is
// an id" would send a code shaped like one to the wrong endpoint and say
// nothing about it. Give one address or the other.

// taskPath builds the base path for a task addressed by id or by code. Exactly
// one of the two must be set; requireOneTaskAddress enforces that first.
//
// Both are values someone typed, so both are escaped: a code containing a slash
// addresses a task rather than a different route.
func taskPath(id, code string) string {
	if code != "" {
		return "/mission/tasks/code/" + url.PathEscape(code)
	}
	return "/mission/tasks/" + url.PathEscape(id)
}

// requireOneTaskAddress exits 5 unless exactly one address was given, and
// unless a code came with a tenant.
//
// A code is unique within a tenant, not across them, and the API refuses a
// code lookup that resolves to no tenant — `X-Tenant-Code header is required
// for code-based task lookup`. Whether it resolves depends on the API key: a
// tenant-scoped key carries one, a global key does not, and the CLI cannot see
// which it holds. So the tenant is demanded here, where the message can say
// why, rather than left to a 400 that arrives for some keys and not others.
func requireOneTaskAddress(id, code string, tenant *string) {
	switch {
	case id != "" && code != "":
		failValidation("give a task id or --code, not both")
	case id == "" && code == "":
		failValidation("a task address is required: an id, or --code <code>")
	}
	if code != "" && tenant == nil {
		failValidation("--code needs a tenant: a task code is unique within a tenant, not across them; pass --tenant <code> or set a default")
	}
}

// splitAttachmentArgs reads the positionals of a download command, which take a
// task id and an attachment id — unless --code supplied the task, in which case
// the single positional is the attachment.
//
// It does not validate: requireOneTaskAddress does that, and reports it.
func splitAttachmentArgs(args []string, code string) (taskID, attachmentID string) {
	if code != "" {
		if len(args) > 0 {
			attachmentID = args[len(args)-1]
		}
		// A second positional alongside --code is a task id, and giving both
		// addresses is the error requireOneTaskAddress exists to name.
		if len(args) > 1 {
			taskID = args[0]
		}
		return taskID, attachmentID
	}
	if len(args) > 0 {
		taskID = args[0]
	}
	if len(args) > 1 {
		attachmentID = args[1]
	}
	return taskID, attachmentID
}

// requireAttachmentID exits 5 when the attachment was never named. Without it a
// missing positional builds `.../attachments//download`, a path the server can
// only answer with a puzzled 404.
func requireAttachmentID(attachmentID string) {
	if attachmentID == "" {
		failValidation("an attachment id is required")
	}
}
