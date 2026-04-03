import { useEffect, useMemo, useState } from 'react'
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'
import { marked } from 'marked'
import { Folder, File, FileText, FileCode, ChevronUp } from 'lucide-react'
import { FileEntry, listFiles, readFileContent } from '../../api/projects'

// ── File type detection ───────────────────────────────────────────────────────

const IMAGE_EXT = /\.(png|jpe?g|gif|svg|webp|ico|avif)$/i
const HTML_EXT  = /\.html?$/i
const MD_EXT    = /\.md$/i

const isImage = (name: string) => IMAGE_EXT.test(name)
const isHtml  = (name: string) => HTML_EXT.test(name)
const isMd    = (name: string) => MD_EXT.test(name)

// Pick a Lucide icon component based on file extension
function FileIcon({ name, className = 'w-3.5 h-3.5 shrink-0' }: { name: string; className?: string }) {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  const isCode = ['ts','tsx','js','jsx','py','go','rs','c','cpp','cs','java','rb','php',
                  'swift','kt','sh','bash','zsh','sql','tf','html','css','Dockerfile'].includes(ext)
  const isText = ['md','txt','csv','json','yaml','yml','toml','xml','log','env','ini'].includes(ext)

  if (isText) return <FileText className={className} />
  if (isCode) return <FileCode className={className} />
  return <File className={className} />
}

// ── Syntax highlighting ───────────────────────────────────────────────────────

const EXT_LANG: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript', js: 'javascript', jsx: 'javascript',
  go: 'go', py: 'python', sh: 'bash', bash: 'bash', zsh: 'bash',
  json: 'json', yaml: 'yaml', yml: 'yaml', toml: 'toml',
  md: 'markdown', html: 'html', css: 'css', sql: 'sql',
  rs: 'rust', java: 'java', c: 'c', cpp: 'cpp', cs: 'csharp',
  rb: 'ruby', php: 'php', swift: 'swift', kt: 'kotlin',
  tf: 'hcl', Dockerfile: 'dockerfile',
}

function langFor(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase() ?? ''
  return EXT_LANG[ext] ?? 'plaintext'
}

function highlight(code: string, lang: string): string {
  try {
    if (lang !== 'plaintext') return hljs.highlight(code, { language: lang }).value
  } catch {}
  return hljs.highlightAuto(code).value
}


// ── Component ─────────────────────────────────────────────────────────────────

interface Props {
  projectId: string
  sessionId: string
}

export default function FileViewer({ projectId, sessionId }: Props) {
  const [entries, setEntries]           = useState<FileEntry[]>([])
  const [path, setPath]                 = useState('')
  const [selected, setSelected]         = useState<string | null>(null)
  const [content, setContent]           = useState<string | null>(null)
  const [viewMode, setViewMode]         = useState<'source' | 'preview'>('source')
  const [loading, setLoading]           = useState(false)
  const [contentLoading, setContentLoading] = useState(false)
  const [error, setError]               = useState<string | null>(null)

  // Load directory listing
  useEffect(() => {
    setLoading(true)
    setSelected(null)
    setContent(null)
    listFiles(projectId, sessionId, path)
      .then(setEntries)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [projectId, sessionId, path])

  const openFile = async (name: string) => {
    const fullPath = path ? `${path}/${name}` : name
    setSelected(fullPath)
    setViewMode('source')
    if (isImage(name)) {
      setContent('__image__')
      return
    }
    setContentLoading(true)
    setContent(null)
    setError(null)
    try {
      const text = await readFileContent(projectId, sessionId, fullPath)
      setContent(text)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setContentLoading(false)
    }
  }

  const breadcrumbs = path.split('/').filter(Boolean)

  // Derived helpers
  const fileName   = selected?.split('/').pop() ?? ''
  const htmlFile   = !!selected && isHtml(fileName)
  const mdFile     = !!selected && isMd(fileName)
  const hasPreview = htmlFile || mdFile
  const isPreviewing = hasPreview && viewMode === 'preview'

  // Rendered markdown (synchronous — avoids an extra useEffect)
  const markdownHtml = useMemo(() => {
    if (!mdFile || !content) return ''
    try {
      const result = marked.parse(content)
      return typeof result === 'string' ? result : ''
    } catch {
      return ''
    }
  }, [content, mdFile])

  return (
    <div className="flex h-full bg-[#F7F4EF]">
      {/* Inject markdown prose styles once */}
      {/* ── File tree ─────────────────────────────────────────────────────── */}
      <div className="w-52 shrink-0 flex flex-col border-r border-[#DDD5C8] overflow-hidden">
        {/* Breadcrumb nav */}
        <div className="px-2 py-1.5 border-b border-[#DDD5C8] flex items-center gap-1 text-[10px] text-[#8A8278] overflow-x-auto whitespace-nowrap">
          <button onClick={() => setPath('')} className="hover:text-[#1C1A16] shrink-0">/</button>
          {breadcrumbs.map((seg, i) => (
            <span key={i} className="flex items-center gap-1 shrink-0">
              <span>/</span>
              <button
                onClick={() => setPath(breadcrumbs.slice(0, i + 1).join('/'))}
                className="hover:text-[#1C1A16]"
              >{seg}</button>
            </span>
          ))}
        </div>

        {/* Entries */}
        <div className="flex-1 overflow-y-auto">
          {loading && <div className="px-3 py-2 text-[10px] text-[#A09888]">Loading…</div>}

          {/* Parent dir row */}
          {path && (
            <button
              onClick={() => setPath(breadcrumbs.slice(0, -1).join('/'))}
              className="w-full text-left px-3 py-1 text-[11px] text-[#8A8278] hover:bg-[#E8E2D8] border-b border-[#DDD5C8] flex items-center gap-1.5"
            >
              <ChevronUp className="w-3 h-3 shrink-0" />
              <span>..</span>
            </button>
          )}

          {entries.map(e => {
            const entryPath = path ? `${path}/${e.name}` : e.name
            return (
              <button
                key={e.name}
                onClick={() => e.isDir ? setPath(entryPath) : openFile(e.name)}
                className={[
                  'w-full text-left px-3 py-1 text-[11px] border-b border-[#DDD5C8] hover:bg-[#E8E2D8] transition-colors flex items-center gap-1.5',
                  selected === entryPath ? 'bg-[#E8E0D4]' : '',
                ].join(' ')}
              >
                {e.isDir ? (
                  <Folder className="w-3.5 h-3.5 shrink-0 text-[#F59E0B]" />
                ) : (
                  <FileIcon name={e.name} className="w-3.5 h-3.5 shrink-0 text-[#8A8278]" />
                )}
                <span className={e.isDir ? 'text-[#F59E0B] truncate' : 'text-[#1C1A16] truncate'}>
                  {e.name}
                </span>
                {!e.isDir && e.size > 0 && (
                  <span className="ml-auto text-[#A09888] text-[9px] shrink-0">
                    {e.size < 1024 ? `${e.size}b` : `${(e.size / 1024).toFixed(1)}k`}
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>

      {/* ── Content pane ──────────────────────────────────────────────────── */}
      <div className={`flex-1 ${isPreviewing ? 'flex flex-col overflow-hidden' : 'overflow-auto'}`}>
        {!selected && (
          <div className="flex items-center justify-center h-full text-[#A09888] text-sm">
            Select a file to view
          </div>
        )}
        {selected && contentLoading && (
          <div className="flex items-center justify-center h-full text-[#A09888] text-sm">
            Loading…
          </div>
        )}
        {selected && error && (
          <div className="p-4 text-[#E53935] text-xs font-mono">{error}</div>
        )}
        {selected && content === '__image__' && (
          <div className="p-4 flex items-start justify-center">
            <img
              src={`/api/projects/${projectId}/sessions/${sessionId}/files/${selected}`}
              alt={selected}
              className="max-w-full rounded border border-[#DDD5C8]"
            />
          </div>
        )}
        {selected && content !== null && content !== '__image__' && (
          <>
            {/* File header */}
            <div className={`flex items-center gap-2 px-3 py-1.5 bg-[#E8E2D8] border-b border-[#DDD5C8] ${isPreviewing ? 'shrink-0' : 'sticky top-0'}`}>
              <span className="text-[10px] text-[#8A8278] font-mono truncate">{selected}</span>

              {hasPreview ? (
                <div className="ml-auto flex gap-1">
                  {(['source', 'preview'] as const).map(mode => (
                    <button
                      key={mode}
                      onClick={() => setViewMode(mode)}
                      className={[
                        'text-[10px] px-2 py-0.5 rounded capitalize',
                        viewMode === mode
                          ? 'bg-[#388bfd]/20 text-[#F59E0B]'
                          : 'bg-[#E8E0D4] text-[#8A8278] hover:text-[#1C1A16]',
                      ].join(' ')}
                    >{mode}</button>
                  ))}
                </div>
              ) : (
                <span className="ml-auto text-[9px] text-[#A09888]">{langFor(fileName)}</span>
              )}
            </div>

            {/* Preview: markdown or HTML */}
            {isPreviewing && mdFile && (
              <div className="flex-1 overflow-auto bg-[#F7F4EF]">
                <div
                  className="md-prose max-w-3xl mx-auto px-8 py-6 text-sm"
                  dangerouslySetInnerHTML={{ __html: markdownHtml }}
                />
              </div>
            )}

            {/* Preview: HTML in iframe */}
            {isPreviewing && htmlFile && (
              <iframe
                srcDoc={content}
                className="flex-1 w-full border-0 bg-[#F7F4EF]"
                sandbox="allow-scripts allow-same-origin"
                title={selected}
              />
            )}

            {/* Source view */}
            {!isPreviewing && (
              <pre className="m-0 p-0 overflow-auto">
                <code
                  className="block p-4 text-[12px] font-mono leading-relaxed"
                  dangerouslySetInnerHTML={{
                    __html: highlight(content, langFor(fileName)),
                  }}
                />
              </pre>
            )}
          </>
        )}
      </div>
    </div>
  )
}
