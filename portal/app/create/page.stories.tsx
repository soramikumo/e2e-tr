import type { Meta, StoryObj } from '@storybook/react';
import CreatePage from './page';

const meta: Meta<typeof CreatePage> = {
  title: 'Pages/Create',
  component: CreatePage,
};

export default meta;
type Story = StoryObj<typeof CreatePage>;

// Shows the idle form before a recording session is started.
export const IdleForm: Story = {};
