import { Routes, Route } from 'react-router-dom'
import './App.css'

function HomePage() {
  return (
    <main>
      <h1>Orion</h1>
      <p>Dashboard home</p>
    </main>
  )
}

function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
    </Routes>
  )
}

export default App
