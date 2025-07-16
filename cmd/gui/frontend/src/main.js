import './style.css'
import './style/normalize.css'
import './style/skeleton.css'

import App from './App.svelte'

const app = new App({
  target: document.getElementById('app')
})

export default app
