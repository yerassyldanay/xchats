import MarkdownIt from 'markdown-it'

// Markdown rendering for assistant answers. The model is instructed to use
// lists and tables when comparing things (see internal/chat's system prompt),
// so rendering it as plain text would throw away most of the structure it was
// asked to produce.
//
// html: false is the security boundary. Answer text is model output derived
// from knowledge-base content an operator authored, so it is not arbitrary
// attacker input — but it is not trusted markup either, and there is no
// reason for this renderer to ever emit a tag the model asked for verbatim.
// With raw HTML disabled, markdown-it escapes it instead, and the output is
// safe to mount with v-html.
const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

// Links open in a new tab: an answer's link points at something outside the
// chat, and losing an in-progress conversation to a navigation would be a
// poor trade. noopener/noreferrer come with it — target="_blank" without them
// hands the opened page a reference back to this one.
const defaultLinkOpen =
  md.renderer.rules.link_open ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  tokens[idx].attrSet('target', '_blank')
  tokens[idx].attrSet('rel', 'noopener noreferrer')
  return defaultLinkOpen(tokens, idx, options, env, self)
}

/** renderMarkdown turns an assistant answer into sanitized HTML. */
export function renderMarkdown(text: string): string {
  return md.render(text)
}
