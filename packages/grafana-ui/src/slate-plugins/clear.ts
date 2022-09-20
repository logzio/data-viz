import { Plugin, Editor as CoreEditor } from 'slate-react';

// Clears the rest of the line after the caret
export function ClearPlugin(): Plugin {
  return {
    onKeyDown(event, editor: CoreEditor, next: Function) {
      const keyEvent = event;
      const value = editor.value;

      if (value.selection.isExpanded) {
        return next();
      }

      if (keyEvent.key === 'k' && keyEvent.ctrlKey) {
        keyEvent.preventDefault();
        const text = value.anchorText.text;
        const offset = value.selection.anchor.offset;
        const length = text.length;
        const forward = length - offset;
        editor.deleteForward(forward);
        return true;
      }

      return next();
    },
  };
}
