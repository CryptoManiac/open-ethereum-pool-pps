import Ember from 'ember';
import config from './config/environment';

var Router = Ember.Router.extend({
  location: config.locationType
});

Router.map(function() {
  this.route('account', { path: '/account/:login' }, function() {
    this.route('shifts');
    this.route('today');
    this.route('payouts');
  });
  this.route('not-found');

  this.route('blocks');

  this.route('help');
  this.route('payments');
  this.route('miners');
  this.route('about');
});

export default Router;
