import React, {useSyncExternalStore} from 'react';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

type EditorPreset = 'modal' | 'standard' | 'emacs';
type Platform = 'darwin' | 'linux';
type BindingValue = string | string[];
type BindingMap = Record<string, BindingValue>;

interface KeybindingData {
  modal: PresetData;
  standard: Record<Platform, PresetData>;
  emacs: PresetData;
}

interface PresetData {
  command_key: string;
  key_bindings: BindingMap;
}

interface Selection {
  preset: EditorPreset;
  platform: Platform;
}

const CHANGE_EVENT = 'runeeditorpresetchange';
const SERVER_SELECTION = 'standard:darwin';

function readSelection(): string {
  if (typeof document === 'undefined') return SERVER_SELECTION;
  const preset = document.documentElement.getAttribute(
    'data-rune-editor-preset',
  );
  const platform = document.documentElement.getAttribute('data-rune-platform');
  const safePreset: EditorPreset =
    preset === 'modal' || preset === 'emacs' ? preset : 'standard';
  const safePlatform: Platform = platform === 'linux' ? 'linux' : 'darwin';
  return `${safePreset}:${safePlatform}`;
}

function subscribe(onChange: () => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  window.addEventListener(CHANGE_EVENT, onChange);
  return () => window.removeEventListener(CHANGE_EVENT, onChange);
}

function useSelection(): Selection {
  const value = useSyncExternalStore(
    subscribe,
    readSelection,
    () => SERVER_SELECTION,
  );
  const [preset, platform] = value.split(':') as [EditorPreset, Platform];
  return {preset, platform};
}

function commandMatches(value: BindingValue, command: BindingValue): boolean {
  if (typeof command === 'string') return value === command;
  return (
    Array.isArray(value) &&
    value.length === command.length &&
    value.every((item, index) => item === command[index])
  );
}

export interface KeyBindingProps {
  command: BindingValue;
}

export default function KeyBinding({command}: KeyBindingProps): React.ReactNode {
  const {siteConfig} = useDocusaurusContext();
  const data = siteConfig.customFields?.keybindings as unknown as KeybindingData;
  const {preset, platform} = useSelection();
  const presetData =
    preset === 'standard' ? data.standard[platform] : data[preset];
  const bindings = presetData.key_bindings;
  const keys = Object.entries(bindings)
    .filter(([, value]) => commandMatches(value, command))
    .map(([key]) => key);

  if (keys.length === 0) {
    return (
      <span
        className="rune-key-binding rune-key-binding--unbound"
        title={typeof command === 'string' ? command : command.join('; ')}>
        not bound by selected preset
      </span>
    );
  }

  return (
    <span
      className="rune-key-binding"
      title={typeof command === 'string' ? command : command.join('; ')}>
      {keys.map((key, index) => (
        <React.Fragment key={key}>
          {index > 0 ? ', ' : null}
          <code>{key}</code>
        </React.Fragment>
      ))}
    </span>
  );
}

export function CommandPromptKey(): React.ReactNode {
  const {siteConfig} = useDocusaurusContext();
  const data = siteConfig.customFields?.keybindings as unknown as KeybindingData;
  const {preset, platform} = useSelection();
  const presetData =
    preset === 'standard' ? data.standard[platform] : data[preset];
  return <code>{presetData.command_key}</code>;
}
