import type { Preview } from "@storybook/angular";
import { setCompodocJson } from "@storybook/addon-docs/angular";
import docJson from "../documentation.json";
import {applicationConfig} from "@storybook/angular";
import {provideRouter, withDisabledInitialNavigation} from '@angular/router';
setCompodocJson(docJson);

const decorators = [
  applicationConfig({
    // Resource cards need routing context without navigating the Storybook iframe URL.
    providers: [provideRouter([], withDisabledInitialNavigation())],
  }),
];

const preview: Preview = {
  parameters: {
    actions: { argTypesRegex: "^on[A-Z].*" },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/,
      },
    },
  },
  decorators: decorators
};

export default preview;
