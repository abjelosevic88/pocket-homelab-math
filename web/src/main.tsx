import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router-dom'
import './styles/base.css'
import './styles/math.css'
import './styles/app.css'
import './styles/settings.css'
import { AppProvider, useApp } from './lib/app-context'
import { Loading, ToastProvider } from './components/ui'
import Layout from './components/Layout'
import { ErrorBoundary } from './components/ErrorBoundary'
import Login from './pages/Login'
import Home from './pages/Home'
import Notebooks from './pages/Notebooks'
import NotebookPage from './pages/NotebookPage'
import Formulas from './pages/Formulas'
import FormulaPage from './pages/FormulaPage'
import Answers from './pages/Answers'
import Convert from './pages/Convert'
import Practice from './pages/Practice'
import Reference from './pages/Reference'
import Shared from './pages/Shared'
import Settings from './pages/Settings'
import { DataProvider } from './lib/data'

function Gate({ children }: { children: React.ReactNode }) {
  const { status, user } = useApp()
  const loc = useLocation()
  if (!status) return <div className="boot"><img src="/favicon.svg" alt="" /><Loading /></div>
  if (status.needs_setup) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace state={{ from: loc.pathname + loc.search }} />
  return <>{children}</>
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login mode="login" />} />
      <Route path="/setup" element={<Login mode="setup" />} />
      <Route path="/s/:key" element={<ErrorBoundary where="share"><Shared /></ErrorBoundary>} />
      <Route path="/settings/:tab?" element={<Gate><ErrorBoundary where="settings"><DataProvider><Settings /></DataProvider></ErrorBoundary></Gate>} />
      <Route element={<Gate><ErrorBoundary where="math"><Layout /></ErrorBoundary></Gate>}>
        <Route path="/" element={<Home />} />
        <Route path="/notebooks" element={<Notebooks />} />
        <Route path="/n/:id" element={<NotebookPage />} />
        <Route path="/formulas" element={<Formulas />} />
        <Route path="/f/:id" element={<FormulaPage />} />
        <Route path="/answers" element={<Answers />} />
        <Route path="/convert" element={<Convert />} />
        <Route path="/practice" element={<Practice />} />
        <Route path="/reference" element={<Reference />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <ErrorBoundary where="app"><ToastProvider><AppProvider><App /></AppProvider></ToastProvider></ErrorBoundary>
    </BrowserRouter>
  </React.StrictMode>,
)
