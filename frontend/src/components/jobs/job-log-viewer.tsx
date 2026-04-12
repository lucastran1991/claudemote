"use client"

// Live log viewer using EventSource (SSE). JWT is passed via ?token= query param
// because EventSource cannot set custom headers. Implements sticky-bottom auto-scroll
// and exponential backoff reconnect (up to 5 attempts).

import { useEffect, useRef, useState, useCallback } from "react"
import { useSession } from "next-auth/react"

const API_BASE = process.env.NEXT_PUBLIC_BACKEND_URL ?? ""
const MAX_LINES = 5000
const MAX_RETRIES = 5

interface JobLogViewerProps {
  jobId: string
  /** When true, the job is terminal and SSE stream will be closed server-side. */
  isTerminal?: boolean
}

export function JobLogViewer({ jobId, isTerminal }: JobLogViewerProps) {
  const { data: session } = useSession()
  const token = session?.accessToken ?? ""

  const [lines, setLines] = useState<string[]>([])
  const [disconnected, setDisconnected] = useState(false)
  const [retryCount, setRetryCount] = useState(0)

  // Refs that don't trigger re-render
  const esRef = useRef<EventSource | null>(null)
  const lastEventIdRef = useRef<string>("")
  const retryCountRef = useRef(0)
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const preRef = useRef<HTMLPreElement>(null)
  const isAtBottomRef = useRef(true)

  const appendLines = useCallback((newLine: string) => {
    // Unescape literal \n sequences the backend encodes in SSE data
    const decoded = newLine.replace(/\\n/g, "\n")
    setLines((prev) => {
      const next = [...prev, decoded]
      return next.length > MAX_LINES ? next.slice(next.length - MAX_LINES) : next
    })
  }, [])

  const connect = useCallback(() => {
    if (!token) return
    // Close any existing connection
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }

    const params = new URLSearchParams({ token })
    if (lastEventIdRef.current) {
      params.set("lastEventId", lastEventIdRef.current)
    }

    const url = `${API_BASE}/api/jobs/${jobId}/stream?${params.toString()}`
    const es = new EventSource(url)
    esRef.current = es

    es.onmessage = (event) => {
      // Reset backoff on successful message
      retryCountRef.current = 0
      setRetryCount(0)
      setDisconnected(false)

      if (event.lastEventId) {
        lastEventIdRef.current = event.lastEventId
      }
      appendLines(event.data)

      // Auto-scroll if at bottom
      if (isAtBottomRef.current && preRef.current) {
        const el = preRef.current
        el.scrollTop = el.scrollHeight
      }
    }

    es.onerror = () => {
      es.close()
      esRef.current = null

      const attempt = retryCountRef.current + 1
      retryCountRef.current = attempt
      setRetryCount(attempt)

      if (attempt > MAX_RETRIES) {
        setDisconnected(true)
        return
      }

      // Exponential backoff: 1s, 2s, 4s, 8s, 16s
      const delay = Math.min(1000 * Math.pow(2, attempt - 1), 16000)
      retryTimerRef.current = setTimeout(() => {
        connect()
      }, delay)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, jobId, appendLines])

  useEffect(() => {
    // isTerminal jobs still connect: backend replays full history then closes the stream.
    // Removing the isTerminal guard ensures completed jobs show their output.
    if (!token) return
    connect()

    return () => {
      esRef.current?.close()
      esRef.current = null
      if (retryTimerRef.current) clearTimeout(retryTimerRef.current)
    }
  }, [token, jobId, connect])

  // Track scroll position to decide whether to auto-scroll
  const handleScroll = useCallback(() => {
    const el = preRef.current
    if (!el) return
    const threshold = 40
    isAtBottomRef.current =
      el.scrollHeight - el.scrollTop - el.clientHeight < threshold
  }, [])

  const handleRetry = useCallback(() => {
    retryCountRef.current = 0
    setRetryCount(0)
    setDisconnected(false)
    connect()
  }, [connect])

  return (
    <div className="rounded-xl border border-white/10 overflow-hidden space-y-0">
      <div className="flex items-center justify-between px-4 py-2 bg-slate-900/80 border-b border-white/10">
        <span className="text-xs text-white/40 font-mono uppercase tracking-wide">
          Output
        </span>
        {retryCount > 0 && retryCount <= MAX_RETRIES && (
          <span className="text-xs text-amber-400">
            Reconnecting… (attempt {retryCount}/{MAX_RETRIES})
          </span>
        )}
      </div>

      {disconnected && (
        <div className="flex items-center justify-between px-4 py-2 bg-red-900/20 border-b border-red-500/20">
          <span className="text-xs text-red-400">
            Stream disconnected after {MAX_RETRIES} attempts.
          </span>
          <button
            onClick={handleRetry}
            className="text-xs text-red-300 underline hover:text-red-200"
          >
            Retry
          </button>
        </div>
      )}

      <pre
        ref={preRef}
        onScroll={handleScroll}
        className="bg-slate-950 text-slate-100 font-mono text-xs p-4 h-96 overflow-y-auto overflow-x-auto whitespace-pre leading-5"
      >
        {lines.length === 0 ? (
          <span className="text-slate-500">Waiting for output…</span>
        ) : (
          lines.join("\n")
        )}
      </pre>
    </div>
  )
}
