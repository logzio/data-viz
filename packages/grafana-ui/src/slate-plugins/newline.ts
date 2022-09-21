import { Plugin, Editor as CoreEditor } from 'slate-react';

function getIndent(text: string) {
  let offset = text.length - text.trimLeft().length;
  if (offset) {
    let indent = text[0];
    while (--offset) {
      indent += text[0];
    }
    return indent;
  }
  return '';
}

export function NewlinePlugin(): Plugin {
  return {
    onKeyDown(event, editor: CoreEditor, next: Function) {
      const keyEvent = event;
      const value = editor.value;

      if (value.selection.isExpanded) {
        return next();
      }

      if (keyEvent.key === 'Enter') {
        keyEvent.preventDefault();

        const { startBlock } = value;
        const currentLineText = startBlock.text;
        const indent = getIndent(currentLineText);

        return editor.splitBlock().insertText(indent).focus();
      }

      return next();
    },
  };
}
