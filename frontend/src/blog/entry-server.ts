import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import ArticleLayout, { type ArticleLayoutProps } from './ArticleLayout.vue'
import ArticleIndex, { type ArticleIndexProps } from './ArticleIndex.vue'
import NotFound, { type NotFoundProps } from './NotFound.vue'

// Pure server rendering, no hydration — build-blog.ts writes this output
// straight to a static .html file and never ships a client bundle alongside it.
export async function renderArticle(props: ArticleLayoutProps): Promise<string> {
  const app = createSSRApp({ render: () => h(ArticleLayout, props) })
  return renderToString(app)
}

export async function renderIndex(props: ArticleIndexProps): Promise<string> {
  const app = createSSRApp({ render: () => h(ArticleIndex, props) })
  return renderToString(app)
}

export async function renderNotFound(props: NotFoundProps): Promise<string> {
  const app = createSSRApp({ render: () => h(NotFound, props) })
  return renderToString(app)
}
