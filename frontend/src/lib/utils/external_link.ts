export function OpenExternalLink(url: string, desktopMode: boolean, windowId?: string){
  //check if url starts with https, and if not, prepend it (external links are never relative)
  if(!url.startsWith("https://") && !url.startsWith("http://")){
    url = "https://" + url;
  }

  //check if wails exists and is defined
  if(desktopMode){
    GetDesktopBridge().then(bridge => bridge.openExternal(url, windowId || 'external')).catch(console.error)
  } else{
    window.open(url, "_blank");
  }

}
import {GetDesktopBridge} from './desktop_bridge';
