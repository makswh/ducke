import './app.css'
import { mount } from 'svelte'
import App from './App.svelte'

// Safe dev-mock when running in a standalone browser outside Wails WebView
if (typeof window !== 'undefined') {
  if (!(window as any).runtime) {
    (window as any).runtime = {
      EventsOn: () => () => {},
      EventsOnMultiple: () => () => {},
      EventsOnce: () => () => {},
      EventsOff: () => {},
      EventsOffAll: () => {},
      EventsEmit: () => {},
      LogPrint: console.log,
      BrowserOpenURL: (url: string) => window.open(url, '_blank'),
      WindowReload: () => window.location.reload(),
    };
  }
  if (!(window as any).go) {
    (window as any).go = { main: { App: {} } };
  }
}

const app = mount(App, {
  target: document.getElementById('app')!
})

export default app
