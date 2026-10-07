import { useCallback, useEffect, useRef, useState } from 'react'

function extensionOf(name) {
  const parts = String(name || '').split('.')
  return parts.length > 1 ? parts.pop().toLowerCase() : ''
}

export function fileMatchesAccept(file, accept) {
  if (!file) return false
  if (!accept) return true

  const wanted = String(accept)
    .split(',')
    .map((a) => a.trim().toLowerCase())
    .filter(Boolean)

  if (!wanted.length) return true

  const ext = '.' + extensionOf(file.name)
  const mime = (file.type || '').toLowerCase()

  return wanted.some((w) => {
    if (w.startsWith('.')) return w === ext
    if (w.endsWith('/*')) return mime.startsWith(w.slice(0, -1))
    return w === mime
  })
}

export function acceptHint(accept) {
  return String(accept || '')
    .split(',')
    .map((a) => a.trim())
    .filter(Boolean)
    .join(', ')
}

export function formatFileSize(bytes) {
  if (!bytes && bytes !== 0) return ''
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + ' KB'
  return (bytes / 1024 / 1024).toFixed(1) + ' MB'
}

const dragListeners = new Set()
let globalDepth = 0
let globalWired = false

function hasFiles(e) {
  return Array.from(e.dataTransfer?.types || []).includes('Files')
}

function broadcast(active) {
  dragListeners.forEach((fn) => fn(active))
  if (document.body) document.body.classList.toggle('is-file-dragging', active)
}

function wireGlobal() {
  if (globalWired || typeof window === 'undefined') return
  globalWired = true

  window.addEventListener('dragenter', (e) => {
    if (!hasFiles(e)) return
    globalDepth += 1
    if (globalDepth === 1) broadcast(true)
  })
  window.addEventListener('dragleave', (e) => {
    if (!hasFiles(e)) return
    globalDepth = Math.max(0, globalDepth - 1)
    if (globalDepth === 0) broadcast(false)
  })
  window.addEventListener('drop', () => {
    globalDepth = 0
    broadcast(false)
  })
  window.addEventListener('dragend', () => {
    globalDepth = 0
    broadcast(false)
  })
}

export function useWindowFileDrag() {
  const [active, setActive] = useState(false)
  useEffect(() => {
    wireGlobal()
    dragListeners.add(setActive)
    return () => {
      dragListeners.delete(setActive)
    }
  }, [])
  return active
}

export default function useFileDrop({ accept = '', disabled = false, onFile, onReject }) {
  const [dragging, setDragging] = useState(false)
  const [rejected, setRejected] = useState(false)
  const depth = useRef(0)
  const rejectTimer = useRef(null)
  const windowDragging = useWindowFileDrag()

  useEffect(
    () => () => {
      if (rejectTimer.current) clearTimeout(rejectTimer.current)
    },
    [],
  )

  const flashReject = useCallback(() => {
    setRejected(true)
    if (rejectTimer.current) clearTimeout(rejectTimer.current)
    rejectTimer.current = setTimeout(() => setRejected(false), 1600)
  }, [])

  const onDragEnter = useCallback(
    (e) => {
      if (disabled || !hasFiles(e)) return
      e.preventDefault()
      depth.current += 1
      setDragging(true)
    },
    [disabled],
  )

  const onDragOver = useCallback(
    (e) => {
      if (disabled || !hasFiles(e)) return
      e.preventDefault()
      if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
      setDragging(true)
    },
    [disabled],
  )

  const onDragLeave = useCallback(
    (e) => {
      if (disabled) return
      e.preventDefault()
      depth.current = Math.max(0, depth.current - 1)
      if (depth.current === 0) setDragging(false)
    },
    [disabled],
  )

  const onDrop = useCallback(
    (e) => {
      if (disabled) return
      e.preventDefault()
      e.stopPropagation()
      depth.current = 0
      setDragging(false)

      const dropped = Array.from(e.dataTransfer?.files || [])
      if (!dropped.length) return

      const file = dropped[0]
      if (!fileMatchesAccept(file, accept)) {
        flashReject()
        if (onReject) onReject(file, acceptHint(accept))
        return
      }
      if (onFile) onFile(file)
    },
    [accept, disabled, flashReject, onFile, onReject],
  )

  const stateClass = [
    dragging ? 'is-drag-over' : '',
    !dragging && windowDragging && !disabled ? 'is-drag-armed' : '',
    rejected ? 'is-drag-rejected' : '',
  ]
    .filter(Boolean)
    .join(' ')

  return {
    dragging,
    rejected,
    windowDragging,
    stateClass,
    dropProps: { onDragEnter, onDragOver, onDragLeave, onDrop },
  }
}
