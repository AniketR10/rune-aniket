import React from 'react';
import MDXComponents from '@theme-original/MDXComponents';
import KeyBinding, {CommandPromptKey} from '@site/src/components/KeyBinding';

export default {
  ...MDXComponents,
  KeyBinding,
  CommandPromptKey,
} satisfies Record<
  string,
  React.ComponentType<never> | keyof React.JSX.IntrinsicElements
>;
