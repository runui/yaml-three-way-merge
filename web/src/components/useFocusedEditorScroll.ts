import { useState } from "react";

export function useFocusedEditorScroll() {
  const [focused, setFocused] = useState(false);
  return {
    focused,
    focusHandlers: {
      onFocusCapture: () => setFocused(true),
      onBlurCapture: () => setFocused(false),
    },
  };
}
