/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchQuery } from 'wander-ui/utils/properties/watch';
import WithWatchers from 'wander-ui/mixins/with-watchers';
import { inject as service } from '@ember/service';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service store;

  startWatchers(controller) {
    controller.set('modelWatch', this.watch.perform({ type: 'csi' }));
  }

  @watchQuery('plugin') watch;
  @collect('watch') watchers;
}
