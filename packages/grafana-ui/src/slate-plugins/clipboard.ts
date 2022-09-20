import { Plugin, Editor as CoreEditor } from 'slate-react';

const getCopiedText = (textBlocks: string[], startOffset: number, endOffset: number) => {
  if (!textBlocks.length) {
    return undefined;
  }

  const excludingLastLineLength = textBlocks.slice(0, -1).join('').length + textBlocks.length - 1;
  return textBlocks.join('\n').slice(startOffset, excludingLastLineLength + endOffset);
};

export function ClipboardPlugin(): Plugin {
  const clipboardPlugin: Plugin = {
    onCopy(event, editor: CoreEditor, next: () => any) {
      const clipEvent = event;
      clipEvent.preventDefault();

      const { document, selection } = editor.value;
      const {
        start: { offset: startOffset },
        end: { offset: endOffset },
      } = selection;
      const selectedBlocks = document
        .getLeafBlocksAtRange(selection)
        .toArray()
        .map((block) => block.text);

      const copiedText = getCopiedText(selectedBlocks, startOffset, endOffset);
      if (copiedText && clipEvent.clipboardData) {
        clipEvent.clipboardData.setData('Text', copiedText);
      }

      return true;
    },

    onPaste(event, editor: CoreEditor, next: () => any) {
      const clipEvent = event;
      clipEvent.preventDefault();
      if (clipEvent.clipboardData) {
        const pastedValue = clipEvent.clipboardData.getData('Text');
        const lines = pastedValue.split('\n');

        if (lines.length) {
          editor.insertText(lines[0]);
          for (const line of lines.slice(1)) {
            editor.splitBlock().insertText(line);
          }
        }
      }

      return true;
    },
  };

  return {
    ...clipboardPlugin,
    onCut(event, editor: CoreEditor, next: () => any) {
      const clipEvent = event;
      clipboardPlugin.onCopy!(clipEvent, editor, next);
      editor.deleteAtRange(editor.value.selection);

      return true;
    },
  };
}
