import { getAllStoredSync, storeActiveGamepadConfig, updateGameName } from './internal/state/chromeStoredData';
import { disableActionButton, enableActionButton } from './internal/utils/actionButtonUtils';
import { DEFAULT_CONFIG_NAME } from './shared/gamepadConfig';
import { MessageTypes, activateGamepadConfigMsg, Message } from './shared/messages';

/*
 * This script is run as a service worker and may be killed or restarted at any time.
 * Make sure to read the following for more information:
 * https://developer.chrome.com/docs/extensions/mv3/migrating_to_service_workers/
 */

console.log('Connecting to native messaging host');
chrome.runtime.connectNative('com.xcloud.xcloud_keyboard_mouse');

chrome.runtime.onInstalled.addListener(({ reason }) => {
  // Page actions are disabled by default and enabled on select tabs
  disableActionButton();
  if (reason === 'install') {
    // First time install - enable the default gamepad config
    storeActiveGamepadConfig(DEFAULT_CONFIG_NAME);
  }
  if (typeof chrome.runtime.setUninstallURL === 'function') {
    chrome.runtime.setUninstallURL('https://forms.gle/nzToDcw1mmssMBLx6');
  }
});

chrome.runtime.onMessage.addListener((msg: Message, sender, sendResponse) => {
  // Receives messages from the content_script
  if (!sender.tab) return false;

  console.log('Connected');
  if (msg.type === MessageTypes.INJECTED) {
    console.log('Injected');
    enableActionButton(sender.tab.id);
    return false;
  }
  if (msg.type === MessageTypes.INITIALIZED) {
    console.log('Initialized', msg.gameName);
    updateGameName(msg.gameName);
    // Send any currently-active config
    getAllStoredSync().then(({ isEnabled, activeConfig, configs }) => {
      const config = !isEnabled ? null : configs[activeConfig];
      sendResponse(activateGamepadConfigMsg(activeConfig, config));
    });
    // https://stackoverflow.com/a/56483156
    return true;
  }
  if (msg.type === MessageTypes.GAME_CHANGED) {
    console.log('Game changed to', msg.gameName);
    updateGameName(msg.gameName);
    return false;
  }
  return false;
});
