import type { Meta, StoryObj } from '@storybook/react';
import Home from './page';

const meta: Meta<typeof Home> = {
  title: 'Pages/Home',
  component: Home,
};

export default meta;
type Story = StoryObj<typeof Home>;

// Shows the empty state before any test tags or scenarios are recorded.
// The fetch calls will fail silently, leaving tags/scenarios as empty arrays.
export const EmptyState: Story = {};
