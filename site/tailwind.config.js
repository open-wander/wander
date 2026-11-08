const wanderUI = require('../ui/tailwind.config.js')

module.exports = {
  ...wanderUI,
  content: [
    './templates/**/*.templ',
    './templates/**/*.go',
  ],
  plugins: wanderUI.plugins,
}
