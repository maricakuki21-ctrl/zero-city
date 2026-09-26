import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import custom from './custom'
import { mergeLocaleMessages } from '../merge'

const officialMessages = {
  ...landing,
  ...common,
  ...dashboard,
  ...batchImage,
  admin,
  ...misc,
}

// Keep official v0.1.160 additions while retaining BizDecipher's customized copy.
export default mergeLocaleMessages(officialMessages, custom)
