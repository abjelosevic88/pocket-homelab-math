// Pocket AI helping: explaining the working, and turning a problem in words
// (or in a photo) into sums. The answers always come from the calculator.
import { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { useAppState } from '../lib/data'
import type { Line } from '../lib/calc'
import { Icon } from './icons'
import { errMsg, Loading, Modal, Segmented, useToast } from './ui'

export function AIExplain({ text, question, angle, onClose }: { text: string; question?: string; angle?: string; onClose: () => void }) {
  const [level, setLevel] = useState<'adult' | 'child'>('adult')
  const [res, setRes] = useState<{ steps: string[]; summary: string; tip: string } | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    setRes(null); setErr('')
    api.post<{ steps: string[]; summary: string; tip: string }>(`${V1}/ai/explain`, { text, question, level, angle }).then(setRes).catch(e => setErr(errMsg(e)))
  }, [text, question, level, angle])
  return <Modal title="Explained" onClose={onClose}>
    <div className="stack">
      <Segmented size="sm" value={level} onChange={setLevel} options={[{ value: 'adult', label: 'For me' }, { value: 'child', label: 'For a child' }]} label="Explain it" />
      {err ? <p className="bad">{err}</p> : !res ? <Loading label="Pocket AI is writing it out…" /> : <>
        <ol className="steps">{(res.steps || []).map((s, i) => <li key={i}>{s}</li>)}</ol>
        {res.summary && <p><b>{res.summary}</b></p>}
        {res.tip && <p className="callout info"><Icon name="lightbulb" size={15} /><span>{res.tip}</span></p>}
      </>}
    </div>
  </Modal>
}

type Out = { title: string; problem: string; text: string; results: Line[]; note: string; graph?: string[]; equation?: string }

export function AIWrite({ onClose, onLines }: { onClose: () => void; onLines: (text: string, title: string, extra?: Out) => void }) {
  const { toast } = useToast()
  const appState = useAppState()
  const [tab, setTab] = useState<'words' | 'photo'>('words')
  const [words, setWords] = useState('')
  const [busy, setBusy] = useState(false)
  const [out, setOut] = useState<Out | null>(null)
  const [photos, setPhotos] = useState<{ id: string; thumb_url: string; name: string }[] | null>(null)
  const photosOK = appState('photos', 'photos:read').ok
  useEffect(() => { if (tab === 'photo' && photosOK && !photos) api.get<{ photos: { id: string; thumb_url: string; name: string }[] }>(`${V1}/apps/photos`).then(r => setPhotos(r.photos)).catch(() => setPhotos([])) }, [tab, photosOK, photos])
  const run = async (url: string, body: unknown) => {
    setBusy(true); setOut(null)
    try { setOut(await api.post<Out>(url, body)) } catch (e) { toast(errMsg(e), 'err') }
    setBusy(false)
  }
  const file = (f: File | undefined) => {
    if (!f) return
    if (f.size > 15 << 20) { toast('That picture is too big (15 MB at most)', 'err'); return }
    const rd = new FileReader()
    rd.onload = () => run(`${V1}/ai/photo`, { image: rd.result })
    rd.readAsDataURL(f)
  }
  return <Modal title="Work it out from words or a photo" onClose={onClose} size="lg" footer={out ? <>
    <button type="button" className="btn" onClick={() => setOut(null)}>Try again</button>
    <button type="button" className="btn primary" onClick={() => onLines(out.text, out.title, out)}>Add to the notebook</button>
  </> : undefined}>
    <div className="stack">
      <Segmented value={tab} onChange={setTab} options={[{ value: 'words', label: 'In words' }, { value: 'photo', label: 'A photo' }]} label="From" />
      {tab === 'words' && <>
        <textarea className="paste-in" rows={5} value={words} onChange={e => setWords(e.target.value)} placeholder="A room is 4.2 by 3.6 m with walls 2.6 m high. One litre of paint covers 10 m². How many 2.5 L tins do I need for two coats?" autoFocus />
        <button type="button" className="btn primary" disabled={busy || !words.trim()} onClick={() => run(`${V1}/ai/write`, { text: words })}><Icon name="sparkles" size={15} />{busy ? 'Writing the sums…' : 'Write the sums'}</button>
      </>}
      {tab === 'photo' && <>
        <label className="btn"><Icon name="camera" size={15} />Take or choose a picture<input type="file" accept="image/*" capture="environment" hidden onChange={e => file(e.target.files?.[0])} /></label>
        {photosOK ? (photos === null ? <Loading /> : photos.length > 0 && <>
          <p className="muted small">Or one from Pocket Photos:</p>
          <div className="photo-grid">{photos.slice(0, 24).map(p => <button key={p.id} type="button" className="photo-pick" onClick={() => run(`${V1}/ai/photo`, { photo_id: p.id })} title={p.name}><img src={p.thumb_url} alt={p.name} loading="lazy" /></button>)}</div>
        </>) : <p className="muted small">Link Pocket Photos (Settings → Connected apps) to pick from your photos.</p>}
        {busy && <Loading label="Reading the picture…" />}
      </>}
      {out && <div className="ai-out">
        {out.problem && <p className="muted">{out.problem}</p>}
        <div className="ai-lines">{out.text.split('\n').map((l, i) => <div key={i} className="ai-line"><code>{l}</code><span className={out.results[i]?.kind === 'error' ? 'bad' : ''}>{out.results[i]?.text || out.results[i]?.error || ''}</span></div>)}</div>
        {out.note && <p className="callout info small"><Icon name="info" size={14} /><span>{out.note}</span></p>}
      </div>}
    </div>
  </Modal>
}
