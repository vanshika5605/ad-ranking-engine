import React, { useState, useEffect } from 'react'

const API = import.meta.env.VITE_API_URL || ''

function App() {
  const [campaigns, setCampaigns] = useState([])
  const [suggestions, setSuggestions] = useState([])
  const [selectedCampaign, setSelectedCampaign] = useState(null)
  const [stats, setStats] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)

  const fetchCampaigns = async () => {
    const url = `${API}/api/campaigns`
    console.log('[Dashboard] fetchCampaigns URL:', url)
    try {
      const r = await fetch(url)
      console.log('[Dashboard] fetch status:', r.status, r.statusText)
      if (!r.ok) throw new Error(r.statusText)
      const data = await r.json()
      console.log('[Dashboard] raw response:', data, 'isArray:', Array.isArray(data), 'type:', typeof data)
      const list = Array.isArray(data) ? data : (data?.data ?? data?.campaigns ?? [])
      const plain = JSON.parse(JSON.stringify(list))
      console.log('[Dashboard] list to set:', plain, 'length:', plain.length)
      setCampaigns(plain)
      setError(null)
    } catch (e) {
      console.error('[Dashboard] fetchCampaigns error:', e)
      setError(e.message)
    }
  }

  const fetchSuggestions = async () => {
    try {
      const r = await fetch(`${API}/api/suggestions`)
      if (!r.ok) return
      const data = await r.json()
      setSuggestions(Array.isArray(data) ? data : [])
    } catch (_) {}
  }

  const fetchCampaignStats = async (id) => {
    setSelectedCampaign(id)
    try {
      const r = await fetch(`${API}/api/campaigns/${id}/stats`)
      if (!r.ok) throw new Error(r.statusText)
      const data = await r.json()
      setStats(Array.isArray(data) ? data : [])
    } catch (e) {
      setStats([])
    }
  }

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      setLoading(true)
      await fetchCampaigns()
      await fetchSuggestions()
      if (!cancelled) setLoading(false)
    })()
    return () => { cancelled = true }
  }, [])

  useEffect(() => {
    if (!selectedCampaign) return
    fetchCampaignStats(selectedCampaign)
  }, [selectedCampaign])

  if (loading) {
    return (
      <div>
        <h1>Ad Ranking Dashboard</h1>
        <div className="loading">Loading campaigns…</div>
      </div>
    )
  }
  if (error) {
    return (
      <div>
        <h1>Ad Ranking Dashboard</h1>
        <div className="error">
          Error: {error}. Is the ad-server running on port 8080? Try: docker compose up -d ad-server
        </div>
        <button type="button" onClick={() => { setError(null); setLoading(true); fetchCampaigns().then(() => setLoading(false)); }}>
          Retry
        </button>
      </div>
    )
  }

  const campaignById = Object.fromEntries((campaigns || []).map((c) => [c.id, c]))

  const showTable = Array.isArray(campaigns) && campaigns.length > 0
  console.log('[Dashboard] render state:', {
    campaigns,
    campaignsLength: campaigns?.length,
    isArray: Array.isArray(campaigns),
    showTable,
  })

  return (
    <>
      <h1>Ad Ranking Dashboard</h1>

      <section>
        <h2>Campaigns</h2>
        {showTable ? (
          <>
            <p style={{ marginBottom: '0.75rem', color: '#94a3b8', fontSize: '0.9rem' }}>
              Showing {campaigns.length} campaign{campaigns.length !== 1 ? 's' : ''}
            </p>
            <table>
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Bid (¢)</th>
                  <th>Impressions</th>
                  <th>Clicks</th>
                  <th className="ctr">CTR %</th>
                  <th>Spend (¢)</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {campaigns.map((c, i) => (
                  <tr key={c.id ?? i}>
                    <td>{c.id ?? '-'}</td>
                    <td>{c.name ?? '-'}</td>
                    <td><span className={`status ${c.status || 'unknown'}`}>{c.status ?? '-'}</span></td>
                    <td>{c.bid_cents ?? c.bidCents ?? 0}</td>
                    <td>{c.impressions ?? 0}</td>
                    <td>{c.clicks ?? 0}</td>
                    <td className="ctr">{(c.impressions ?? 0) > 0 ? (Number(c.ctr) || 0).toFixed(2) : '-'}</td>
                    <td>{c.spend_cents ?? c.spendCents ?? 0}</td>
                    <td>
                      <button type="button" onClick={() => fetchCampaignStats(c.id)}>Stats</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <p style={{ marginTop: '0.5rem' }}>
              <button type="button" onClick={() => { fetchCampaigns(); fetchSuggestions(); }}>Refresh</button>
            </p>
          </>
        ) : (
          <p className="loading">
            No campaigns. Ensure ad-server is running and the DB has seed data (restart ad-server to run migrations + seed).
          </p>
        )}
      </section>

      {selectedCampaign && campaignById[selectedCampaign] != null && (
        <section>
          <h2>Daily stats: {campaignById[selectedCampaign]?.name ?? 'Campaign ' + selectedCampaign}</h2>
          <table className="stats-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>Impressions</th>
                <th>Clicks</th>
                <th>Spend (¢)</th>
              </tr>
            </thead>
            <tbody>
              {(stats || []).length === 0 ? (
                <tr><td colSpan={4}>No daily data yet. Run the simulator to generate traffic.</td></tr>
              ) : (
                (stats || []).map((d, i) => (
                  <tr key={d.date ?? i}>
                    <td>{d.date ?? '-'}</td>
                    <td>{d.impressions ?? 0}</td>
                    <td>{d.clicks ?? 0}</td>
                    <td>{d.spend_cents ?? d.spendCents ?? 0}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </section>
      )}

      <section>
        <h2>Suggestions</h2>
        {(suggestions || []).length === 0 ? (
          <p className="loading">
            No suggestions yet. Suggestions appear when campaigns have enough traffic (e.g. low CTR + high spend, or high CTR). Run the simulator to generate more data.
          </p>
        ) : (
          <ul className="suggestions-list">
            {(suggestions || []).map((s, i) => (
              <li key={i}>
                <div className="type">{s.type}</div>
                {s.message}
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  )
}

export default App
