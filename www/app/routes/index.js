import Ember from 'ember';
import config from '../config/environment';

export default Ember.Route.extend({
  actions: {
    lookup(login) {
      if (!Ember.isEmpty(login)) {
        return this.transitionTo('account', login);
      }
    },
    calculate(hashRate) {
      if (!Ember.isEmpty(hashRate)) {
        var url = '//' + window.location.hostname + '/api/stats';
        Ember.$.getJSON(url).then(function(data) {

            var result = '<div class="col-md-12"><br/><div class="note note-info">';
            result += '<span>';
            result += '<p>Расчет для скорости <span class="label label-success">' + hashRate + '</span> MH/s и сложности <span class="label label-success">' +  data.nodes[0].difficulty + '</span>:</p>';

            result += '<dl class="dl-horizontal">';
            var intervals = [1, 3, 7, 14, 30];
            var ppsRate = 3 * (1 - config.APP.PoolFee / 100) / data.nodes[0].difficulty;
            for (var i = 0; i < intervals.length; ++i) {
                var seconds = intervals[i] * 86400;
                var hashes = hashRate * Math.pow(10, 6) * seconds;
                result += '<dt>' + intervals[i] + 'дн.</dt>';
                result += '<dd>' + (hashes * ppsRate).toFixed(8)  +  ' ETH</dd>';
            }

            result += '</dl>';
            result += '</span>';
            result += '</div></div>';

            document.getElementById('calculation-result').innerHTML = result;
        });

      }
    }
  }
});
