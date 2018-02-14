import Ember from 'ember';
import config from '../config/environment';

export default Ember.Controller.extend({
  applicationController: Ember.inject.controller('application'),
  stats: Ember.computed.reads('applicationController.model.stats'),
  est24hr2: Ember.computed('stats', 'model', {
    get() {
      var diff = Ember.$.cookie('difficulty');
      if (diff) {
        return this.get('model.hashrate') * 86400 * 2955000000 / diff;
      }
      return 0;
     }
   })
});
