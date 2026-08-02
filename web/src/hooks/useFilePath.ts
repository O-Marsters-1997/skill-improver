import { useCallback, useSyncExternalStore } from "react";

// pushState fires no event of its own, so navigate() announces itself. popstate covers the
// back and forward buttons.
const NAVIGATED = "app:navigated";

function subscribe(onChange: () => void) {
  window.addEventListener("popstate", onChange);
  window.addEventListener(NAVIGATED, onChange);
  return () => {
    window.removeEventListener("popstate", onChange);
    window.removeEventListener(NAVIGATED, onChange);
  };
}

// The URL path is the file: /references/theming.md reviews references/theming.md, and "/"
// means whichever file the server picks first. Encoding is per segment so a filename with a
// space survives the round trip while the slashes stay slashes.
function read() {
  return decodeURIComponent(window.location.pathname).replace(/^\//, "");
}

export function useFilePath(): [string, (rel: string) => void] {
  const rel = useSyncExternalStore(subscribe, read);

  const navigate = useCallback(
    (next: string) => {
      if (next === rel) return;
      const url = `/${next.split("/").map(encodeURIComponent).join("/")}`;
      window.history.pushState(null, "", url);
      window.dispatchEvent(new Event(NAVIGATED));
    },
    [rel],
  );

  return [rel, navigate];
}
