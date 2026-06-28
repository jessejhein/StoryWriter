export type Health = { status: string; version: string }

export type Project = {
  project_id: string
  name?: string
  path: string
  git_initialized: boolean
  index_initialized: boolean
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init)
  const body = await response.json()
  if (!response.ok) {
    throw new Error(body.error ?? `Request failed with status ${response.status}`)
  }
  return body as T
}

export function getHealth(): Promise<Health> {
  return request('/api/health')
}

export function createProject(name: string, path: string): Promise<Project> {
  return request('/api/projects', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, path }),
  })
}

export function openProject(path: string): Promise<Project> {
  return request('/api/projects/open', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
}
